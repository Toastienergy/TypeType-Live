package chat

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	DefaultURL   = "wss://irc-ws.chat.twitch.tv"
	historySize  = 50
	idleGrace    = 30 * time.Second
	subscriberCh = 64
)

type Hub struct {
	URL   string
	mu    sync.Mutex
	rooms map[string]*room
}

func NewHub(url string) *Hub {
	if url == "" {
		url = DefaultURL
	}
	return &Hub{URL: url, rooms: make(map[string]*room)}
}

type room struct {
	hub     *Hub
	channel string
	mu      sync.Mutex
	subs    map[chan Event]struct{}
	history []Event
	cancel  context.CancelFunc
	idle    *time.Timer
}

func (h *Hub) Subscribe(channel string) (<-chan Event, []Event, func()) {
	h.mu.Lock()
	r := h.rooms[channel]
	if r == nil {
		r = &room{hub: h, channel: channel, subs: make(map[chan Event]struct{})}
		ctx, cancel := context.WithCancel(context.Background())
		r.cancel = cancel
		h.rooms[channel] = r
		go r.run(ctx)
	}
	h.mu.Unlock()

	ch := make(chan Event, subscriberCh)
	r.mu.Lock()
	if r.idle != nil {
		r.idle.Stop()
		r.idle = nil
	}
	r.subs[ch] = struct{}{}
	history := append([]Event(nil), r.history...)
	r.mu.Unlock()
	return ch, history, func() { r.leave(ch) }
}

func (h *Hub) Rooms() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms)
}

func (r *room) leave(ch chan Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subs, ch)
	if len(r.subs) == 0 && r.idle == nil {
		r.idle = time.AfterFunc(idleGrace, r.close)
	}
}

func (r *room) close() {
	r.hub.mu.Lock()
	r.mu.Lock()
	if len(r.subs) > 0 {
		r.mu.Unlock()
		r.hub.mu.Unlock()
		return
	}
	delete(r.hub.rooms, r.channel)
	r.mu.Unlock()
	r.hub.mu.Unlock()
	r.cancel()
}

func (r *room) broadcast(ev Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = append(r.history, ev)
	if len(r.history) > historySize {
		r.history = r.history[len(r.history)-historySize:]
	}
	for ch := range r.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (r *room) run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := r.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		slog.Debug("chat reconnect", "channel", r.channel, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (r *room) session(ctx context.Context) error {
	conn, resp, err := websocket.Dial(ctx, r.hub.URL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	for _, cmd := range []string{
		"CAP REQ :twitch.tv/tags twitch.tv/commands",
		fmt.Sprintf("NICK justinfan%d", 1000+n.Int64()),
		"JOIN #" + r.channel,
	} {
		if err := conn.Write(ctx, websocket.MessageText, []byte(cmd)); err != nil {
			return err
		}
	}
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		for raw := range strings.SplitSeq(strings.TrimRight(string(data), "\r\n"), "\r\n") {
			if strings.HasPrefix(raw, "PING") {
				_ = conn.Write(ctx, websocket.MessageText, []byte("PONG"+strings.TrimPrefix(raw, "PING")))
				continue
			}
			if ev, ok := parseLine(raw).event(); ok {
				r.broadcast(ev)
			}
		}
	}
}

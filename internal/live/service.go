package live

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/TypeType-Video/TypeType-Live/cache"
	"github.com/TypeType-Video/TypeType-Live/hls"
	"github.com/TypeType-Video/TypeType-Live/proxy"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

const (
	KindLive = "live"
	KindVOD  = "vod"

	masterLinkTTL    = 12 * time.Hour
	prepareTimeout   = 15 * time.Second
	variantLinkTTL   = 12 * time.Hour
	liveSegmentTTL   = 10 * time.Minute
	vodSegmentTTL    = 8 * time.Hour
	livePlaylistTTL  = time.Second
	vodPlaylistTTL   = 10 * time.Minute
	tokenSafety      = 2 * time.Minute
	maxPlaylistBytes = 2 << 20
	maxSegmentBytes  = 24 << 20
	liveSegmentCache = 12 * time.Second
)

var (
	ErrUnknownSession = errors.New("live: unknown session")
	ErrUnknownVariant = errors.New("live: unknown variant")
)

var DefaultBackupPlayers = []twitch.Player{
	{Type: "autoplay", Platform: "android"},
	{Type: "embed", Platform: "web"},
	{Type: "popout", Platform: "web"},
}

type Config struct {
	Twitch        *twitch.Client
	Relay         *proxy.Relay
	Signer        *proxy.Signer
	Sealer        *proxy.Sealer
	SegmentBytes  int64
	BackupPlayers []twitch.Player
	Metrics       *Metrics
}

type Service struct {
	twitch        *twitch.Client
	relay         *proxy.Relay
	signer        *proxy.Signer
	sealer        *proxy.Sealer
	backupPlayers []twitch.Player
	metrics       *Metrics
	now           func() time.Time

	segments      *cache.LRU
	playlists     *cache.LRU
	segmentGroup  cache.Group[[]byte]
	playlistGroup cache.Group[[]byte]
	masterGroup   cache.Group[*masterState]

	mu       sync.Mutex
	masters  map[string]*masterState
	sessions *sessions
}

func New(cfg Config) *Service {
	metrics := cfg.Metrics
	if metrics == nil {
		metrics = &Metrics{}
	}
	return &Service{
		twitch:        cfg.Twitch,
		relay:         cfg.Relay,
		signer:        cfg.Signer,
		sealer:        cfg.Sealer,
		backupPlayers: cfg.BackupPlayers,
		metrics:       metrics,
		now:           time.Now,
		segments:      cache.NewLRU(cfg.SegmentBytes),
		playlists:     cache.NewLRU(8 << 20),
		masters:       make(map[string]*masterState),
		sessions:      newSessions(),
	}
}

func (s *Service) SegmentStats() cache.Stats { return s.segments.Stats() }

type masterState struct {
	master     hls.Master
	expires    time.Time
	restricted bool
}

func (s *Service) PrepareMaster(ctx context.Context, kind, id, userToken string) string {
	link, key := s.masterLink(kind, id, userToken)
	background := context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(background, prepareTimeout)
		defer cancel()
		_, _ = s.masterState(ctx, kind, id, key, twitch.DefaultPlayer, false)
	}()
	return link
}

func (s *Service) masterLink(kind, id, userToken string) (string, string) {
	values := url.Values{"t": {kind}, "c": {id}}
	key := ""
	if userToken != "" {
		key = s.sessions.put(userToken, masterLinkTTL)
		values.Set("k", key)
	}
	return "master.m3u8?" + s.signer.Sign(values, masterLinkTTL).Encode(), key
}

func (s *Service) SignedClipLink(upstream string) string {
	return "clip.mp4?" + s.signer.Sign(url.Values{"x": {s.sealer.Seal(upstream)}}, vodSegmentTTL).Encode()
}

func (s *Service) Verify(values url.Values) error {
	return s.signer.Verify(values)
}

func masterKey(kind, id, session string) string {
	return kind + "/" + id + "/" + session
}

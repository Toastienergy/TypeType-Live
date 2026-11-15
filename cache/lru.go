package cache

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key     string
	value   []byte
	expires time.Time
}

type LRU struct {
	mu       sync.Mutex
	maxBytes int64
	used     int64
	order    *list.List
	items    map[string]*list.Element
	now      func() time.Time
	hits     uint64
	misses   uint64
}

func NewLRU(maxBytes int64) *LRU {
	return &LRU{
		maxBytes: maxBytes,
		order:    list.New(),
		items:    make(map[string]*list.Element),
		now:      time.Now,
	}
}

func (c *LRU) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		c.misses++
		return nil, false
	}
	e := el.Value.(*entry)
	if !e.expires.IsZero() && c.now().After(e.expires) {
		c.remove(el)
		c.misses++
		return nil, false
	}
	c.order.MoveToFront(el)
	c.hits++
	return e.value, true
}

func (c *LRU) Set(key string, value []byte, ttl time.Duration) {
	size := int64(len(value))
	if size > c.maxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.remove(el)
	}
	var expires time.Time
	if ttl > 0 {
		expires = c.now().Add(ttl)
	}
	c.items[key] = c.order.PushFront(&entry{key: key, value: value, expires: expires})
	c.used += size
	now := c.now()
	for back := c.order.Back(); back != nil; back = c.order.Back() {
		e := back.Value.(*entry)
		if c.used <= c.maxBytes && (e.expires.IsZero() || now.Before(e.expires)) {
			break
		}
		c.remove(back)
	}
}

func (c *LRU) remove(el *list.Element) {
	e := c.order.Remove(el).(*entry)
	delete(c.items, e.key)
	c.used -= int64(len(e.value))
}

type Stats struct {
	Entries int
	Bytes   int64
	Hits    uint64
	Misses  uint64
}

func (c *LRU) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{Entries: len(c.items), Bytes: c.used, Hits: c.hits, Misses: c.misses}
}

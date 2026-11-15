package cache

import (
	"context"
	"sync"
)

type call[T any] struct {
	done  chan struct{}
	value T
	err   error
}

type Group[T any] struct {
	mu    sync.Mutex
	calls map[string]*call[T]
}

func (g *Group[T]) Do(ctx context.Context, key string, fn func(context.Context) (T, error)) (T, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*call[T])
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		return wait(ctx, c)
	}
	c := &call[T]{done: make(chan struct{})}
	g.calls[key] = c
	g.mu.Unlock()

	go func() {
		c.value, c.err = fn(context.WithoutCancel(ctx))
		g.mu.Lock()
		delete(g.calls, key)
		g.mu.Unlock()
		close(c.done)
	}()
	return wait(ctx, c)
}

func wait[T any](ctx context.Context, c *call[T]) (T, error) {
	select {
	case <-c.done:
		return c.value, c.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

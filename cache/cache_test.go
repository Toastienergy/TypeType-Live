package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLRUEvictsByBytes(t *testing.T) {
	c := NewLRU(10)
	c.Set("a", make([]byte, 4), 0)
	c.Set("b", make([]byte, 4), 0)
	c.Get("a")
	c.Set("c", make([]byte, 4), 0)
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should stay, it was used recently")
	}
	if s := c.Stats(); s.Bytes != 8 || s.Entries != 2 {
		t.Fatalf("stats %+v", s)
	}
	c.Set("huge", make([]byte, 11), 0)
	if _, ok := c.Get("huge"); ok {
		t.Fatal("values above the cap are not stored")
	}
}

func TestLRUExpires(t *testing.T) {
	c := NewLRU(100)
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	c.Set("a", []byte("x"), time.Second)
	now = now.Add(2 * time.Second)
	if _, ok := c.Get("a"); ok {
		t.Fatal("expired entry returned")
	}
	if c.Stats().Bytes != 0 {
		t.Fatal("expired entry not released")
	}
}

func TestGroupCollapsesConcurrentCalls(t *testing.T) {
	var g Group[int]
	var calls atomic.Int32
	release := make(chan struct{})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			v, err := g.Do(context.Background(), "k", func(context.Context) (int, error) {
				calls.Add(1)
				<-release
				return 7, nil
			})
			if err != nil || v != 7 {
				t.Errorf("v=%d err=%v", v, err)
			}
		})
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestGroupCallerCancellationDoesNotCancelOthers(t *testing.T) {
	var g Group[int]
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := g.Do(ctx, "k", func(c context.Context) (int, error) {
			<-release
			return 1, c.Err()
		})
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	other := make(chan int, 1)
	go func() {
		v, _ := g.Do(context.Background(), "k", func(context.Context) (int, error) { return 2, nil })
		other <- v
	}()
	close(release)
	if v := <-other; v != 1 && v != 2 {
		t.Fatalf("v = %d", v)
	}
}

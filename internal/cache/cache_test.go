package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time          { return f.t }
func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func newTest(ttl time.Duration, max int) (*Memory[int], *fakeClock) {
	c := New[int](ttl, max)
	fc := &fakeClock{t: time.Unix(1_000_000, 0)}
	c.now = fc.now
	return c, fc
}

func TestExpiry(t *testing.T) {
	c, fc := newTest(time.Minute, 10)
	c.mu.Lock()
	c.setLocked("a", 1)
	c.mu.Unlock()
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("want hit, got %v %v", v, ok)
	}
	fc.advance(time.Minute)
	if _, ok := c.Get("a"); ok {
		t.Fatal("entry should have expired")
	}
	if c.Stats().Entries != 0 {
		t.Fatal("an expired entry should be dropped when it is seen")
	}
}

func TestLRUEvictionAndStats(t *testing.T) {
	c, _ := newTest(time.Hour, 2)
	ctx := context.Background()
	load := func(v int) func(context.Context) (int, error) {
		return func(context.Context) (int, error) { return v, nil }
	}
	c.GetOrLoad(ctx, "a", load(1))
	c.GetOrLoad(ctx, "b", load(2))
	c.GetOrLoad(ctx, "a", load(0)) // hit: a becomes most recently used
	c.GetOrLoad(ctx, "c", load(3)) // evicts b

	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should survive")
	}
	s := c.Stats()
	if s.Evictions != 1 || s.Entries != 2 || s.Hits < 1 || s.Misses != 3 {
		t.Fatalf("stats: %+v", s)
	}
}

func TestGetOrLoadCachesAndCounts(t *testing.T) {
	c, _ := newTest(time.Hour, 10)
	loads := 0
	load := func(context.Context) (int, error) { loads++; return 42, nil }
	for i := 0; i < 4; i++ {
		v, err := c.GetOrLoad(context.Background(), "k", load)
		if err != nil || v != 42 {
			t.Fatalf("got %v %v", v, err)
		}
	}
	s := c.Stats()
	if loads != 1 || s.Hits != 3 || s.Misses != 1 {
		t.Fatalf("loads=%d stats=%+v", loads, s)
	}
	if r := s.HitRatio(); r != 0.75 {
		t.Fatalf("hit ratio %v", r)
	}
	if (Stats{}).HitRatio() != 0 {
		t.Fatal("empty stats have ratio 0")
	}
}

func TestErrorsAreNotCached(t *testing.T) {
	c, _ := newTest(time.Hour, 10)
	boom := errors.New("boom")
	if _, err := c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) { return 0, boom }); !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	v, err := c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) { return 7, nil })
	if err != nil || v != 7 {
		t.Fatalf("second load must run: %v %v", v, err)
	}
	if c.Stats().LoadErrors != 1 {
		t.Fatalf("load errors: %+v", c.Stats())
	}
}

func TestZeroTTLNeverServesFromCache(t *testing.T) {
	c, _ := newTest(0, 10)
	loads := 0
	load := func(context.Context) (int, error) { loads++; return 1, nil }
	c.GetOrLoad(context.Background(), "k", load)
	c.GetOrLoad(context.Background(), "k", load)
	if loads != 2 {
		t.Fatalf("with no ttl every lookup loads, loads=%d", loads)
	}
}

func TestSingleFlight(t *testing.T) {
	c, _ := newTest(time.Hour, 10)
	var loads atomic.Int32
	release := make(chan struct{})
	load := func(context.Context) (int, error) {
		loads.Add(1)
		<-release
		return 5, nil
	}

	const callers = 20
	var wg sync.WaitGroup
	results := make(chan int, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.GetOrLoad(context.Background(), "hot", load)
			if err != nil {
				t.Error(err)
			}
			results <- v
		}()
	}
	for loads.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // let the rest queue behind the leader
	close(release)
	wg.Wait()
	close(results)

	if n := loads.Load(); n != 1 {
		t.Fatalf("want 1 load for %d concurrent callers, got %d", callers, n)
	}
	for v := range results {
		if v != 5 {
			t.Fatalf("got %d", v)
		}
	}
	if s := c.Stats(); s.Misses != 1 || s.Hits != callers-1 {
		t.Fatalf("stats: %+v", s)
	}
}

func TestClearDuringLoadDoesNotStoreStaleValue(t *testing.T) {
	c, _ := newTest(time.Hour, 10)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) {
			close(started)
			<-release
			return 1, nil
		})
	}()
	<-started
	c.Clear() // a write happened while the query was running
	close(release)
	<-done
	if _, ok := c.Get("k"); ok {
		t.Fatal("a value loaded before Clear must not be stored")
	}
}

func TestLoaderPanicReleasesWaiters(t *testing.T) {
	c, _ := newTest(time.Hour, 10)
	started, release := make(chan struct{}), make(chan struct{})
	leaderDone := make(chan any, 1)
	go func() {
		defer func() { leaderDone <- recover() }()
		c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) {
			close(started)
			<-release
			panic("boom")
		})
	}()
	<-started

	followerErr := make(chan error, 1)
	go func() {
		_, err := c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) { return 9, nil })
		followerErr <- err
	}()
	time.Sleep(20 * time.Millisecond) // follower queues behind the leader
	close(release)

	if r := <-leaderDone; r == nil {
		t.Fatal("the panic should still reach the leader's caller")
	}
	select {
	case err := <-followerErr:
		if !errors.Is(err, ErrLoadPanicked) {
			t.Fatalf("follower should get ErrLoadPanicked, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follower is stuck behind a panicked loader")
	}
	// and the key is usable again
	v, err := c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) { return 3, nil })
	if err != nil || v != 3 {
		t.Fatalf("key should load normally afterwards: %v %v", v, err)
	}
}

func TestFollowerRetriesWhenLeaderContextEnds(t *testing.T) {
	c, _ := newTest(time.Hour, 10)
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	started := make(chan struct{})
	leaderDone := make(chan error, 1)
	go func() {
		_, err := c.GetOrLoad(leaderCtx, "k", func(ctx context.Context) (int, error) {
			close(started)
			<-ctx.Done()
			return 0, ctx.Err()
		})
		leaderDone <- err
	}()
	<-started

	followerDone := make(chan int, 1)
	go func() {
		v, err := c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) { return 9, nil })
		if err != nil {
			t.Error(err)
		}
		followerDone <- v
	}()
	time.Sleep(20 * time.Millisecond)
	cancelLeader()

	if err := <-leaderDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader: %v", err)
	}
	select {
	case v := <-followerDone:
		if v != 9 {
			t.Fatalf("follower got %d", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follower did not finish")
	}
}

func TestFollowerStopsWaitingWhenItsOwnContextEnds(t *testing.T) {
	c, _ := newTest(time.Hour, 10)
	started, release := make(chan struct{}), make(chan struct{})
	go func() {
		c.GetOrLoad(context.Background(), "k", func(context.Context) (int, error) {
			close(started)
			<-release
			return 1, nil
		})
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := c.GetOrLoad(ctx, "k", func(context.Context) (int, error) { return 2, nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}
	close(release)
}

func TestConcurrentMixedUse(t *testing.T) { // for the race detector
	c := New[int](time.Millisecond, 8)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				key := string(rune('a' + (i+g)%12))
				c.GetOrLoad(context.Background(), key, func(context.Context) (int, error) { return i, nil })
				if i%97 == 0 {
					c.Clear()
				}
				c.Stats()
			}
		}(g)
	}
	wg.Wait()
}

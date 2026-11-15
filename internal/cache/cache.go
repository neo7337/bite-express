// Package cache is a small in-memory read cache: bounded (LRU), expiring (TTL) and
// stampede-safe (concurrent requests for the same missing key share one load).
//
// It is written against the Provider interface so that a shared cache such as Redis can
// replace it later without touching the code that uses it.
package cache

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Provider is what callers depend on. Memory implements it; a Redis backed cache would
// implement the same two methods (encoding V itself).
type Provider[V any] interface {
	// GetOrLoad returns the cached value for key, or calls load, stores the result and
	// returns it. Load errors are returned and never cached.
	GetOrLoad(ctx context.Context, key string, load func(context.Context) (V, error)) (V, error)
	// Clear drops every entry (used after writes that could change cached results).
	Clear()
}

// Stats are cumulative counters since the cache was created.
type Stats struct {
	Hits       uint64 // served from the cache, or from another caller's load already in flight
	Misses     uint64 // had to run the loader
	LoadErrors uint64 // loaders that failed (the error is not cached)
	Evictions  uint64 // entries pushed out because the cache was full
	Entries    int    // entries held right now
}

// HitRatio is hits / (hits + misses), or 0 when nothing was looked up yet.
func (s Stats) HitRatio() float64 {
	if total := s.Hits + s.Misses; total > 0 {
		return float64(s.Hits) / float64(total)
	}
	return 0
}

// ErrLoadPanicked is what callers waiting on a load see if that load panicked.
var ErrLoadPanicked = errors.New("cache: loader panicked")

// Memory is an in-process Provider. Safe for concurrent use; create it with New.
type Memory[V any] struct {
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	hits, misses, loadErrors, evictions atomic.Uint64

	mu       sync.Mutex
	gen      uint64 // bumped by Clear: a load that began before a Clear is not stored
	items    map[string]*list.Element
	lru      *list.List // front = most recently used
	inflight map[string]*call[V]
}

var _ Provider[int] = (*Memory[int])(nil)

type entry[V any] struct {
	key     string
	value   V
	expires time.Time
}

type call[V any] struct {
	done  chan struct{}
	value V
	err   error
}

// New returns a cache holding at most maxEntries values (minimum 1), each valid for ttl.
// A ttl of zero or less means entries are never served (every lookup loads).
func New[V any](ttl time.Duration, maxEntries int) *Memory[V] {
	if maxEntries < 1 {
		maxEntries = 1
	}
	return &Memory[V]{
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        time.Now,
		items:      make(map[string]*list.Element),
		lru:        list.New(),
		inflight:   make(map[string]*call[V]),
	}
}

// Stats returns a snapshot of the counters.
func (c *Memory[V]) Stats() Stats {
	c.mu.Lock()
	n := c.lru.Len()
	c.mu.Unlock()
	return Stats{
		Hits: c.hits.Load(), Misses: c.misses.Load(),
		LoadErrors: c.loadErrors.Load(), Evictions: c.evictions.Load(), Entries: n,
	}
}

// Get returns a fresh cached value without loading.
func (c *Memory[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getLocked(key)
}

func (c *Memory[V]) getLocked(key string) (v V, ok bool) {
	el, found := c.items[key]
	if !found {
		return v, false
	}
	e := el.Value.(*entry[V])
	if !c.now().Before(e.expires) {
		c.removeLocked(el)
		return v, false
	}
	c.lru.MoveToFront(el)
	return e.value, true
}

func (c *Memory[V]) setLocked(key string, v V) {
	if c.ttl <= 0 {
		return
	}
	if el, ok := c.items[key]; ok {
		e := el.Value.(*entry[V])
		e.value, e.expires = v, c.now().Add(c.ttl)
		c.lru.MoveToFront(el)
		return
	}
	for c.lru.Len() >= c.maxEntries {
		c.removeLocked(c.lru.Back())
		c.evictions.Add(1)
	}
	c.items[key] = c.lru.PushFront(&entry[V]{key: key, value: v, expires: c.now().Add(c.ttl)})
}

func (c *Memory[V]) removeLocked(el *list.Element) {
	delete(c.items, el.Value.(*entry[V]).key)
	c.lru.Remove(el)
}

// Clear drops every entry. Loads already running finish for their own callers, but their
// results are not stored, so a value read before a write cannot outlive the Clear.
func (c *Memory[V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	c.items = make(map[string]*list.Element)
	c.lru.Init()
}

// GetOrLoad implements Provider.
//
// Concurrent calls for the same key share one load. If another caller's load fails only
// because that caller's context ended, and this caller's context is still alive, this
// caller runs the load itself instead of inheriting someone else's cancellation.
func (c *Memory[V]) GetOrLoad(ctx context.Context, key string, load func(context.Context) (V, error)) (V, error) {
	for attempt := 0; ; attempt++ {
		c.mu.Lock()
		if v, ok := c.getLocked(key); ok {
			c.mu.Unlock()
			c.hits.Add(1)
			return v, nil
		}
		if cl, ok := c.inflight[key]; ok { // someone is already loading this key
			c.mu.Unlock()
			select {
			case <-cl.done:
			case <-ctx.Done():
				var zero V
				return zero, ctx.Err()
			}
			if cl.err == nil {
				c.hits.Add(1)
				return cl.value, nil
			}
			if attempt == 0 && isContextErr(cl.err) && ctx.Err() == nil {
				continue // the leader gave up, we have not
			}
			var zero V
			return zero, cl.err
		}

		cl := &call[V]{done: make(chan struct{}), err: ErrLoadPanicked}
		c.inflight[key] = cl
		gen := c.gen
		c.mu.Unlock()

		c.misses.Add(1)
		c.runLoad(ctx, key, gen, cl, load)
		return cl.value, cl.err
	}
}

// runLoad runs the loader and always releases the waiters, even if the loader panics
// (cl.err stays ErrLoadPanicked and the panic continues to the caller).
func (c *Memory[V]) runLoad(ctx context.Context, key string, gen uint64, cl *call[V], load func(context.Context) (V, error)) {
	defer func() {
		c.mu.Lock()
		delete(c.inflight, key)
		if cl.err == nil && gen == c.gen {
			c.setLocked(key, cl.value)
		}
		c.mu.Unlock()
		if cl.err != nil {
			c.loadErrors.Add(1)
		}
		close(cl.done)
	}()
	cl.value, cl.err = load(ctx)
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// Package cached decorates a RestaurantsRepository with a read cache.
//
// Why: at peak hours many clients ask for the same thing (top restaurants in a city, the
// same restaurant page). With the cache a burst of identical queries costs one MongoDB
// query per TTL, and identical queries that arrive together share a single query in flight.
//
// Trade-offs, on purpose:
//   - Results can be up to TTL old. Creating a restaurant clears the cached lists of this
//     instance; the by-id cache needs no clearing because ids are never reused.
//   - It is per process. Several instances each keep their own copy, so after a create the
//     other instances catch up when their TTL expires. A shared cache (Redis) would fix that;
//     the decorator only needs a cache.Provider, so it can be swapped in later.
//   - The key contains the exact coordinates, so geo queries only hit when two clients send
//     identical lat/lng. City and cuisine listings, the heavy hitters, hit well.
//   - The cache is bounded by entry count, not bytes. A list entry holds up to 100
//     restaurants, so keep MaxEntries modest (the default of 1000 is roughly 40 MB for
//     typical 20-item pages).
//   - Not-found results and errors are never cached.
package cached

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/neo7337/bite-express/internal/cache"
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
)

const (
	DefaultTTL        = 60 * time.Second
	DefaultMaxEntries = 1000
	byIDFactor        = 10 // by-id entries are small, so the cache may hold more of them
)

// Options configures New. Zero values pick the defaults above.
type Options struct {
	TTL        time.Duration
	MaxEntries int
}

// Repo implements repository.RestaurantsRepository.
type Repo struct {
	next repository.RestaurantsRepository
	list cache.Provider[repository.ListResult]
	byID cache.Provider[*pkg.Restaurant]
}

var _ repository.RestaurantsRepository = (*Repo)(nil)

// New wraps next with in-memory caches.
func New(next repository.RestaurantsRepository, o Options) *Repo {
	if o.TTL <= 0 {
		o.TTL = DefaultTTL
	}
	if o.MaxEntries <= 0 {
		o.MaxEntries = DefaultMaxEntries
	}
	return NewWithProviders(next,
		cache.New[repository.ListResult](o.TTL, o.MaxEntries),
		cache.New[*pkg.Restaurant](o.TTL, o.MaxEntries*byIDFactor))
}

// NewWithProviders wraps next with any cache implementation (the seam for Redis).
func NewWithProviders(next repository.RestaurantsRepository, list cache.Provider[repository.ListResult], byID cache.Provider[*pkg.Restaurant]) *Repo {
	return &Repo{next: next, list: list, byID: byID}
}

func (r *Repo) List(ctx context.Context, f repository.ListFilters) (repository.ListResult, error) {
	key, err := listKey(f)
	if err != nil { // cannot happen for plain data, but never fail a request because of the cache
		return r.next.List(ctx, f)
	}
	res, err := r.list.GetOrLoad(ctx, key, func(ctx context.Context) (repository.ListResult, error) {
		return r.next.List(ctx, f)
	})
	if err != nil {
		return repository.ListResult{}, err
	}
	// cached values are shared between requests: hand out a copy of the slice so a caller
	// that sorts or edits its result cannot change what the next caller sees
	return repository.ListResult{Items: append([]repository.ListItem(nil), res.Items...), HasMore: res.HasMore}, nil
}

func (r *Repo) FindRestaurantById(ctx context.Context, tenantID, id string) (*pkg.Restaurant, error) {
	found, err := r.byID.GetOrLoad(ctx, tenantID+"/"+id, func(ctx context.Context) (*pkg.Restaurant, error) {
		return r.next.FindRestaurantById(ctx, tenantID, id) // misses and errors are not cached
	})
	if err != nil || found == nil {
		return found, err
	}
	cp := *found
	return &cp, nil
}

// Insert writes through and drops the cached lists, since the new restaurant can belong in
// any of them.
func (r *Repo) Insert(ctx context.Context, rest *pkg.Restaurant) (*pkg.Restaurant, error) {
	created, err := r.next.Insert(ctx, rest)
	if err == nil {
		r.list.Clear()
	}
	return created, err
}

// Vocabulary is not cached here; the service keeps it in memory already.
func (r *Repo) Vocabulary(ctx context.Context, tenantID string) (repository.Vocabulary, error) {
	return r.next.Vocabulary(ctx, tenantID)
}

type statser interface{ Stats() cache.Stats }

// Stats returns the counters of the list and by-id caches (zero for a provider without stats).
func (r *Repo) Stats() (list, byID cache.Stats) {
	if s, ok := r.list.(statser); ok {
		list = s.Stats()
	}
	if s, ok := r.byID.(statser); ok {
		byID = s.Stats()
	}
	return list, byID
}

// StartStatsLogger logs the cache counters every interval, but only when there was cache
// traffic since the last line. It is how you check, without extra tooling, that the cache is
// being hit. Call the returned function to stop it.
func (r *Repo) StartStatsLogger(interval time.Duration, logf func(format string, args ...any)) (stop func()) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		var last uint64
		for {
			select {
			case <-done:
				return
			case <-t.C:
				l, b := r.Stats()
				if total := l.Hits + l.Misses + b.Hits + b.Misses; total != last {
					last = total
					logf("cache stats: list hit_ratio=%.0f%% hits=%d misses=%d evictions=%d entries=%d | by_id hit_ratio=%.0f%% hits=%d misses=%d entries=%d",
						l.HitRatio()*100, l.Hits, l.Misses, l.Evictions, l.Entries,
						b.HitRatio()*100, b.Hits, b.Misses, b.Entries)
				}
			}
		}
	}()
	return func() { close(done) }
}

// listKey hashes every filter field, so two requests share an entry only when they ask the
// same question.
func listKey(f repository.ListFilters) (string, error) {
	b, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

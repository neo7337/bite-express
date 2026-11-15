package cached

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neo7337/bite-express/internal/cache"
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeRepo struct {
	listCalls, findCalls, insertCalls atomic.Int32
	gate                              chan struct{} // when set, List blocks until it is closed
	findErr                           error
}

func (f *fakeRepo) List(ctx context.Context, lf repository.ListFilters) (repository.ListResult, error) {
	f.listCalls.Add(1)
	if f.gate != nil {
		<-f.gate
	}
	return repository.ListResult{Items: []repository.ListItem{{Restaurant: pkg.Restaurant{Name: lf.City}}}}, nil
}

func (f *fakeRepo) FindRestaurantById(ctx context.Context, tenant, id string) (*pkg.Restaurant, error) {
	f.findCalls.Add(1)
	if f.findErr != nil {
		return nil, f.findErr
	}
	return &pkg.Restaurant{ID: bson.NewObjectID(), Name: id}, nil
}

func (f *fakeRepo) Insert(ctx context.Context, r *pkg.Restaurant) (*pkg.Restaurant, error) {
	f.insertCalls.Add(1)
	return r, nil
}

func (f *fakeRepo) Vocabulary(context.Context, string) (repository.Vocabulary, error) {
	return repository.Vocabulary{Cities: []string{"gurgaon"}}, nil
}

func TestListIsCachedPerFilter(t *testing.T) {
	fr := &fakeRepo{}
	r := New(fr, Options{TTL: time.Minute})
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := r.List(ctx, repository.ListFilters{TenantID: "t", City: "Gurgaon"}); err != nil {
			t.Fatal(err)
		}
	}
	if n := fr.listCalls.Load(); n != 1 {
		t.Fatalf("identical filters should hit the database once, got %d", n)
	}

	res, _ := r.List(ctx, repository.ListFilters{TenantID: "t", City: "Delhi"})
	if fr.listCalls.Load() != 2 || res.Items[0].Name != "Delhi" {
		t.Fatal("different filters must not share an entry")
	}
	r.List(ctx, repository.ListFilters{TenantID: "other", City: "Gurgaon"})
	if fr.listCalls.Load() != 3 {
		t.Fatal("tenants must not share an entry")
	}

	near := func(radius float64) repository.ListFilters {
		return repository.ListFilters{TenantID: "t", Near: &repository.GeoFilter{Lat: 28.46, Lng: 77.03, RadiusMeters: radius}}
	}
	r.List(ctx, near(1000))
	r.List(ctx, near(1000))
	r.List(ctx, near(2000))
	if fr.listCalls.Load() != 5 {
		t.Fatalf("pointer fields must be part of the key by value, calls=%d", fr.listCalls.Load())
	}
}

func TestCallerCannotCorruptTheCache(t *testing.T) {
	r := New(&fakeRepo{}, Options{})
	f := repository.ListFilters{TenantID: "t", City: "Gurgaon"}

	first, _ := r.List(context.Background(), f)
	first.Items[0].Name = "mutated by a caller"
	second, _ := r.List(context.Background(), f)
	if second.Items[0].Name != "Gurgaon" {
		t.Fatalf("a later caller saw an earlier caller's edit: %q", second.Items[0].Name)
	}

	a, _ := r.FindRestaurantById(context.Background(), "t", "abc")
	a.Name = "mutated"
	b, _ := r.FindRestaurantById(context.Background(), "t", "abc")
	if b.Name != "abc" {
		t.Fatalf("by-id result must be a copy, got %q", b.Name)
	}
}

func TestFindByIDCachedAndMissesNotCached(t *testing.T) {
	fr := &fakeRepo{}
	r := New(fr, Options{})
	ctx := context.Background()
	r.FindRestaurantById(ctx, "t", "abc")
	r.FindRestaurantById(ctx, "t", "abc")
	if fr.findCalls.Load() != 1 {
		t.Fatalf("found restaurants are cached, calls=%d", fr.findCalls.Load())
	}

	fr.findErr = repository.ErrNotFound
	for i := 0; i < 2; i++ {
		if _, err := r.FindRestaurantById(ctx, "t", "missing"); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	}
	if fr.findCalls.Load() != 3 {
		t.Fatalf("a not-found must not be cached (it may be created next), calls=%d", fr.findCalls.Load())
	}
}

func TestInsertClearsListCache(t *testing.T) {
	fr := &fakeRepo{}
	r := New(fr, Options{})
	ctx := context.Background()
	f := repository.ListFilters{TenantID: "t", City: "Gurgaon"}

	r.List(ctx, f)
	r.List(ctx, f)
	if fr.listCalls.Load() != 1 {
		t.Fatal("precondition: second list is a hit")
	}
	if _, err := r.Insert(ctx, &pkg.Restaurant{Name: "new"}); err != nil {
		t.Fatal(err)
	}
	r.List(ctx, f)
	if fr.listCalls.Load() != 2 {
		t.Fatal("a new restaurant must invalidate cached lists")
	}
}

func TestConcurrentIdenticalListsShareOneQuery(t *testing.T) {
	fr := &fakeRepo{gate: make(chan struct{})}
	r := New(fr, Options{})
	f := repository.ListFilters{TenantID: "t", City: "Gurgaon"}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.List(context.Background(), f); err != nil {
				t.Error(err)
			}
		}()
	}
	for fr.listCalls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)
	close(fr.gate)
	wg.Wait()
	if n := fr.listCalls.Load(); n != 1 {
		t.Fatalf("30 concurrent identical requests should cost 1 query, got %d", n)
	}
}

func TestStats(t *testing.T) {
	r := New(&fakeRepo{}, Options{})
	f := repository.ListFilters{TenantID: "t"}
	r.List(context.Background(), f)
	r.List(context.Background(), f)
	r.FindRestaurantById(context.Background(), "t", "x")

	list, byID := r.Stats()
	if list.Hits != 1 || list.Misses != 1 || list.Entries != 1 {
		t.Fatalf("list stats: %+v", list)
	}
	if byID.Misses != 1 || byID.Entries != 1 {
		t.Fatalf("by-id stats: %+v", byID)
	}
}

func TestStatsLoggerLogsOnlyWhenThereIsTraffic(t *testing.T) {
	r := New(&fakeRepo{}, Options{})
	var mu sync.Mutex
	var lines []string
	stop := r.StartStatsLogger(10*time.Millisecond, func(f string, a ...any) {
		mu.Lock()
		lines = append(lines, f)
		mu.Unlock()
	})
	defer stop()

	time.Sleep(60 * time.Millisecond)
	mu.Lock()
	idle := len(lines)
	mu.Unlock()
	if idle != 0 {
		t.Fatalf("no traffic, no log lines, got %d", idle)
	}

	r.List(context.Background(), repository.ListFilters{TenantID: "t"})
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(lines)
		mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(60 * time.Millisecond) // and it must not repeat the same numbers
	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 1 || !strings.Contains(lines[0], "cache stats") {
		t.Fatalf("want exactly one stats line after traffic, got %v", lines)
	}
}

// fakeProvider shows any cache can be plugged in (this is how a Redis cache would be).
type fakeProvider[V any] struct{ calls, clears int }

func (p *fakeProvider[V]) GetOrLoad(ctx context.Context, _ string, load func(context.Context) (V, error)) (V, error) {
	p.calls++
	return load(ctx)
}
func (p *fakeProvider[V]) Clear() { p.clears++ }

func TestCustomProviders(t *testing.T) {
	list := &fakeProvider[repository.ListResult]{}
	byID := &fakeProvider[*pkg.Restaurant]{}
	r := NewWithProviders(&fakeRepo{}, list, byID)

	r.List(context.Background(), repository.ListFilters{TenantID: "t"})
	r.FindRestaurantById(context.Background(), "t", "x")
	r.Insert(context.Background(), &pkg.Restaurant{Name: "n"})
	if list.calls != 1 || byID.calls != 1 || list.clears != 1 || byID.clears != 0 {
		t.Fatalf("list=%+v byID=%+v", list, byID)
	}
	if l, b := r.Stats(); l != (cache.Stats{}) || b != (cache.Stats{}) {
		t.Fatal("a provider without stats reports zero stats")
	}
}

func TestVocabularyPassesThrough(t *testing.T) {
	v, err := New(&fakeRepo{}, Options{}).Vocabulary(context.Background(), "t")
	if err != nil || len(v.Cities) != 1 {
		t.Fatalf("%v %v", v, err)
	}
}

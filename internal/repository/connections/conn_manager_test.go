package connections

import (
	"context"
	"testing"

	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/internal/repository/cached"
	"github.com/neo7337/bite-express/pkg"
)

type stubRepo struct{}

func (stubRepo) Insert(context.Context, *pkg.Restaurant) (*pkg.Restaurant, error) { return nil, nil }
func (stubRepo) FindRestaurantById(context.Context, string, string) (*pkg.Restaurant, error) {
	return nil, nil
}
func (stubRepo) List(context.Context, repository.ListFilters) (repository.ListResult, error) {
	return repository.ListResult{}, nil
}
func (stubRepo) Vocabulary(context.Context, string) (repository.Vocabulary, error) {
	return repository.Vocabulary{}, nil
}

func TestWithCache(t *testing.T) {
	orig := config.GetConfig
	t.Cleanup(func() { config.GetConfig = orig })
	use := func(c *config.Config) { config.GetConfig = func() *config.Config { return c } }
	inner := stubRepo{}

	use(&config.Config{Cache: config.Cache{Enabled: false}})
	if got := withCache(inner); got != repository.RestaurantsRepository(inner) {
		t.Fatalf("disabled cache must return the repository untouched, got %T", got)
	}

	use(nil)
	if got := withCache(inner); got != repository.RestaurantsRepository(inner) {
		t.Fatalf("no config must not enable the cache, got %T", got)
	}

	use(&config.Config{Cache: config.Cache{Enabled: true, Type: "redis"}})
	if got := withCache(inner); got != repository.RestaurantsRepository(inner) {
		t.Fatalf("an unsupported type must fall back to no cache, got %T", got)
	}

	for _, typ := range []string{"", "memory"} {
		use(&config.Config{Cache: config.Cache{Enabled: true, Type: typ, Config: config.CacheConfig{Ttl: 30, MaxEntries: 10}}})
		if _, ok := withCache(inner).(*cached.Repo); !ok {
			t.Fatalf("type %q should give the in-memory cache", typ)
		}
	}
}

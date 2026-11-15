//go:build integration

package mongodb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
)

const itTenant = "it-tenant"

// newTestRepo connects to the MongoDB at MONGO_URI (default credentials match
// docker-compose.yml) and uses a throwaway database that is dropped on cleanup.
func newTestRepo(t *testing.T) *MongoDBRestaurantsRepo {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI not set; run `make test-integration`")
	}

	conn := config.Connection{
		Uri:        uri,
		DbName:     fmt.Sprintf("biteexpress_it_%d", time.Now().UnixNano()),
		AuthSource: "admin",
		Username:   envOr("MONGO_USER", "admin"),
		Password:   envOr("MONGO_PASSWORD", "secretpassword"),
	}
	base, err := NewMongoDBRepo().Connect(conn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = base.Database.Drop(context.Background())
		_ = base.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := base.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	repo, err := NewMongoDBRestaurantsRepo(base)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	return repo
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func restaurant(sourceID int64, name, city string, cuisines []string, lat, lng, rating float64) *pkg.Restaurant {
	return &pkg.Restaurant{
		TenantID:    itTenant,
		SourceID:    sourceID,
		Name:        name,
		Address:     pkg.Address{Line: "1 Test Road", City: city, Country: "India", CountryCode: 1},
		Location:    &pkg.GeoPoint{Type: "Point", Coordinates: []float64{lng, lat}}, // [lng, lat]
		Cuisines:    cuisines,
		CuisineKeys: lowerAll(cuisines),
		Rating:      pkg.Rating{Average: &rating, Votes: 10},
	}
}

func lowerAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToLower(s)
	}
	return out
}

func TestIntegration_InsertAndFindByID(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	created, err := repo.Insert(ctx, restaurant(1, "Dosa Corner", "Gurgaon", []string{"South Indian"}, 28.4595, 77.0266, 4.2))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if created.ID.IsZero() {
		t.Fatal("expected a generated id")
	}

	got, err := repo.FindRestaurantById(ctx, itTenant, created.ID.Hex())
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Name != "Dosa Corner" || got.Address.City != "Gurgaon" {
		t.Errorf("unexpected restaurant: %+v", got)
	}

	if _, err := repo.FindRestaurantById(ctx, "other-tenant", created.ID.Hex()); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("other tenant: got %v, want ErrNotFound", err)
	}
}

func TestIntegration_DuplicateSourceIDConflicts(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if _, err := repo.Insert(ctx, restaurant(7, "A", "Gurgaon", []string{"Cafe"}, 28.46, 77.03, 4)); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := repo.Insert(ctx, restaurant(7, "B", "Gurgaon", []string{"Cafe"}, 28.47, 77.04, 3))
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
}

func TestIntegration_ListFiltersAndSorting(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	seed := []*pkg.Restaurant{
		restaurant(1, "Near Dosa", "Gurgaon", []string{"South Indian"}, 28.4600, 77.0270, 3.9),
		restaurant(2, "Far Dosa", "Gurgaon", []string{"South Indian"}, 28.5200, 77.1000, 4.6),
		restaurant(3, "Delhi Pizza", "New Delhi", []string{"Italian"}, 28.6139, 77.2090, 4.1),
	}
	for _, r := range seed {
		if _, err := repo.Insert(ctx, r); err != nil {
			t.Fatalf("seed %s: %v", r.Name, err)
		}
	}

	t.Run("filter by city and cuisine, best rated first", func(t *testing.T) {
		res, err := repo.List(ctx, repository.ListFilters{
			TenantID: itTenant, City: "Gurgaon", Cuisines: []string{"south indian"},
			Sort: repository.SortRating, Desc: true, Limit: 10,
		})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(res.Items) != 2 || res.Items[0].Name != "Far Dosa" {
			t.Fatalf("got %+v, want [Far Dosa, Near Dosa]", names(res.Items))
		}
	})

	t.Run("nearest first with distance", func(t *testing.T) {
		res, err := repo.List(ctx, repository.ListFilters{
			TenantID: itTenant,
			Near:     &repository.GeoFilter{Lat: 28.4595, Lng: 77.0266},
			Sort:     repository.SortDistance, Limit: 10,
		})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(res.Items) != 3 || res.Items[0].Name != "Near Dosa" {
			t.Fatalf("got %v, want Near Dosa first", names(res.Items))
		}
		for i, it := range res.Items {
			if it.DistanceMeters == nil {
				t.Fatalf("item %d has no distance", i)
			}
			if i > 0 && *it.DistanceMeters < *res.Items[i-1].DistanceMeters {
				t.Errorf("distances not ascending at %d", i)
			}
		}
	})

	t.Run("radius excludes far places", func(t *testing.T) {
		res, err := repo.List(ctx, repository.ListFilters{
			TenantID: itTenant,
			Near:     &repository.GeoFilter{Lat: 28.4595, Lng: 77.0266, RadiusMeters: 3000},
			Sort:     repository.SortDistance, Limit: 10,
		})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(res.Items) != 1 || res.Items[0].Name != "Near Dosa" {
			t.Fatalf("got %v, want only Near Dosa", names(res.Items))
		}
	})

	t.Run("pagination reports hasMore", func(t *testing.T) {
		res, err := repo.List(ctx, repository.ListFilters{
			TenantID: itTenant, Sort: repository.SortName, Limit: 2,
		})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(res.Items) != 2 || !res.HasMore {
			t.Fatalf("got %d items hasMore=%v, want 2 and true", len(res.Items), res.HasMore)
		}
	})

	t.Run("vocabulary", func(t *testing.T) {
		v, err := repo.Vocabulary(ctx, itTenant)
		if err != nil {
			t.Fatalf("vocabulary: %v", err)
		}
		if len(v.Cities) != 2 || len(v.Cuisines) != 2 {
			t.Errorf("got %+v, want 2 cities and 2 cuisines", v)
		}
	})
}

func names(items []repository.ListItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

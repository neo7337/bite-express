package service

import (
	"context"
	"errors"
	"testing"

	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
)

type fakeRepo struct {
	gotTenant  string
	gotID      string
	gotFilters repository.ListFilters
	inserted   *pkg.Restaurant
	err        error

	vocab      repository.Vocabulary
	vocabErrs  []error // returned in order, one per Vocabulary call
	vocabCalls int
	listCalls  int
}

func (f *fakeRepo) Insert(_ context.Context, r *pkg.Restaurant) (*pkg.Restaurant, error) {
	f.inserted = r
	return r, f.err
}

func (f *fakeRepo) FindRestaurantById(_ context.Context, tenantID, id string) (*pkg.Restaurant, error) {
	f.gotTenant, f.gotID = tenantID, id
	return &pkg.Restaurant{Name: "x"}, f.err
}

func (f *fakeRepo) Vocabulary(context.Context, string) (repository.Vocabulary, error) {
	f.vocabCalls++
	if len(f.vocabErrs) >= f.vocabCalls && f.vocabErrs[f.vocabCalls-1] != nil {
		return repository.Vocabulary{}, f.vocabErrs[f.vocabCalls-1]
	}
	return f.vocab, nil
}

func (f *fakeRepo) List(_ context.Context, lf repository.ListFilters) (repository.ListResult, error) {
	f.listCalls++
	f.gotFilters = lf
	return repository.ListResult{}, f.err
}

func newSvc(repo repository.RestaurantsRepository) *RestaurantsService {
	return &RestaurantsService{repo: repo, config: &config.Config{System: &config.System{TenantID: "tenant-a"}}}
}

func TestCreateRestaurant_ForcesConfiguredTenant(t *testing.T) {
	repo := &fakeRepo{}
	in := &pkg.Restaurant{Name: "x", TenantID: "evil-tenant"} // even if a caller tried to set it
	if _, err := newSvc(repo).CreateRestaurant(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if repo.inserted == nil || repo.inserted.TenantID != "tenant-a" {
		t.Fatalf("tenant = %+v", repo.inserted)
	}
}

func TestCreateRestaurant_PropagatesRepoError(t *testing.T) {
	repo := &fakeRepo{err: repository.ErrConflict}
	if _, err := newSvc(repo).CreateRestaurant(context.Background(), &pkg.Restaurant{}); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestGetRestaurantById_UsesConfiguredTenant(t *testing.T) {
	repo := &fakeRepo{}
	if _, err := newSvc(repo).GetRestaurantById(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	if repo.gotTenant != "tenant-a" || repo.gotID != "abc" {
		t.Fatalf("tenant=%q id=%q", repo.gotTenant, repo.gotID)
	}
}

func TestGetRestaurants_DefaultsTenantButKeepsExplicit(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo)
	_, _ = svc.GetRestaurants(context.Background(), repository.ListFilters{City: "Gurgaon"})
	if repo.gotFilters.TenantID != "tenant-a" || repo.gotFilters.City != "Gurgaon" {
		t.Errorf("filters = %+v", repo.gotFilters)
	}
	_, _ = svc.GetRestaurants(context.Background(), repository.ListFilters{TenantID: "tenant-b"})
	if repo.gotFilters.TenantID != "tenant-b" {
		t.Errorf("explicit tenant overwritten: %+v", repo.gotFilters)
	}
}

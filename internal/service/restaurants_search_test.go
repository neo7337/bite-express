package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/internal/search/nlq"
)

func searchRepo() *fakeRepo {
	return &fakeRepo{vocab: repository.Vocabulary{
		Cities:   []string{"Gurgaon", "Delhi"},
		Cuisines: []string{"south indian", "cafe", "street food"},
	}}
}

func TestSearch_SpecExample_BestInCityWithCuisine(t *testing.T) {
	repo := searchRepo()
	_, intent, err := newSvc(repo).SearchRestaurants(context.Background(), "Best restaurants in Gurgaon with South Indian Food", repository.ListFilters{})
	if err != nil {
		t.Fatal(err)
	}
	f := repo.gotFilters
	if f.TenantID != "tenant-a" || f.City != "gurgaon" || !reflect.DeepEqual(f.Cuisines, []string{"south indian"}) {
		t.Errorf("filters: %+v", f)
	}
	if f.Sort != repository.SortRating || !f.Desc || f.MinVotes == nil || *f.MinVotes != 50 {
		t.Errorf("sort/votes: %+v", f)
	}
	if !intent.Best {
		t.Errorf("intent not returned: %+v", intent)
	}
}

func TestSearch_SpecExample_PriceForTwo(t *testing.T) {
	repo := searchRepo()
	if _, _, err := newSvc(repo).SearchRestaurants(context.Background(), "Restaurants in Gurgaon under Rs. 1000 for two", repository.ListFilters{}); err != nil {
		t.Fatal(err)
	}
	f := repo.gotFilters
	if f.City != "gurgaon" || f.MaxCost == nil || *f.MaxCost != 1000 || f.Currency != "INR" || f.Sort != "" {
		t.Errorf("filters: %+v", f)
	}
}

func TestSearch_SpecExample_NearestNeedsLocation(t *testing.T) {
	repo := searchRepo()
	_, _, err := newSvc(repo).SearchRestaurants(context.Background(), "Nearest place that serves Vada Pav", repository.ListFilters{})
	if !errors.Is(err, repository.ErrInvalidFilter) {
		t.Fatalf("err = %v, want ErrInvalidFilter", err)
	}
	if repo.listCalls != 0 {
		t.Error("the repository must not be queried when the request is invalid")
	}
}

func TestSearch_NearestWithLocationSortsByDistance(t *testing.T) {
	repo := searchRepo()
	near := &repository.GeoFilter{Lat: 28.4, Lng: 77.0}
	if _, _, err := newSvc(repo).SearchRestaurants(context.Background(), "Nearest place that serves Vada Pav", repository.ListFilters{Near: near}); err != nil {
		t.Fatal(err)
	}
	f := repo.gotFilters
	if f.Sort != repository.SortDistance || f.Near != near || !reflect.DeepEqual(f.Cuisines, []string{"street food"}) {
		t.Errorf("filters: %+v", f)
	}
}

func TestSearch_ExplicitFiltersWin(t *testing.T) {
	repo := searchRepo()
	cost := 900
	explicit := repository.ListFilters{City: "Delhi", MaxCost: &cost, Sort: repository.SortName}
	if _, _, err := newSvc(repo).SearchRestaurants(context.Background(), "best cafes under rs 500 in gurgaon", explicit); err != nil {
		t.Fatal(err)
	}
	f := repo.gotFilters
	if f.City != "Delhi" || *f.MaxCost != 900 || f.Sort != repository.SortName || f.Desc {
		t.Errorf("explicit values were overridden: %+v", f)
	}
	if !reflect.DeepEqual(f.Cuisines, []string{"cafe"}) { // not given explicitly, so q fills it
		t.Errorf("cuisines: %v", f.Cuisines)
	}
}

func TestSearch_EmptyQueryBehavesLikeList(t *testing.T) {
	repo := searchRepo()
	if _, _, err := newSvc(repo).SearchRestaurants(context.Background(), "", repository.ListFilters{City: "Delhi"}); err != nil {
		t.Fatal(err)
	}
	if repo.vocabCalls != 0 {
		t.Error("vocabulary should not be loaded without a query")
	}
	if f := repo.gotFilters; f.City != "Delhi" || f.TenantID != "tenant-a" || f.Sort != "" {
		t.Errorf("filters: %+v", f)
	}
}

func TestSearch_VocabularyLoadedOnce(t *testing.T) {
	repo := searchRepo()
	svc := newSvc(repo)
	for i := 0; i < 3; i++ {
		if _, _, err := svc.SearchRestaurants(context.Background(), "cafes in gurgaon", repository.ListFilters{}); err != nil {
			t.Fatal(err)
		}
	}
	if repo.vocabCalls != 1 {
		t.Errorf("vocabulary loaded %d times, want 1", repo.vocabCalls)
	}
}

func TestSearch_VocabularyFailureIsNotCached(t *testing.T) {
	repo := searchRepo()
	repo.vocabErrs = []error{errors.New("mongo down")}
	svc := newSvc(repo)
	if _, _, err := svc.SearchRestaurants(context.Background(), "cafes", repository.ListFilters{}); err == nil {
		t.Fatal("first search should fail")
	}
	if _, _, err := svc.SearchRestaurants(context.Background(), "cafes", repository.ListFilters{}); err != nil {
		t.Fatalf("second search should recover: %v", err)
	}
	if repo.vocabCalls != 2 {
		t.Errorf("vocabCalls = %d, want 2", repo.vocabCalls)
	}
}

func TestSearch_RepositoryErrorPropagates(t *testing.T) {
	repo := searchRepo()
	repo.err = errors.New("mongo down")
	if _, _, err := newSvc(repo).SearchRestaurants(context.Background(), "cafes", repository.ListFilters{}); err == nil {
		t.Fatal("list failure must surface")
	}
}

// ---- mergeIntent (pure) ---------------------------------------------------------------

func TestMergeIntent_NoIntentLeavesFiltersUntouched(t *testing.T) {
	rating := 4.0
	in := repository.ListFilters{City: "x", MinRating: &rating, Sort: repository.SortName, Desc: true, Limit: 5}
	out, err := mergeIntent(in, nlq.Intent{})
	if err != nil || !reflect.DeepEqual(in, out) {
		t.Fatalf("got %+v, %v", out, err)
	}
}

func TestMergeIntent_FillsOnlyEmptyFields(t *testing.T) {
	cost, rating := 700, 4.2
	intent := nlq.Intent{City: "gurgaon", Cuisines: []string{"cafe"}, MaxCost: &cost, Currency: "INR", MinRating: &rating, Text: "pizza hut"}

	out, _ := mergeIntent(repository.ListFilters{}, intent)
	if out.City != "gurgaon" || out.MaxCost != &cost || out.Currency != "INR" || out.MinRating != &rating || out.Text != "pizza hut" ||
		!reflect.DeepEqual(out.Cuisines, []string{"cafe"}) {
		t.Errorf("empty filters should be filled: %+v", out)
	}

	otherCost, otherRating := 100, 3.0
	explicit := repository.ListFilters{City: "delhi", Cuisines: []string{"chinese"}, MaxCost: &otherCost, Currency: "USD", MinRating: &otherRating, Text: "x"}
	out, _ = mergeIntent(explicit, intent)
	if out.City != "delhi" || out.MaxCost != &otherCost || out.Currency != "USD" || out.MinRating != &otherRating || out.Text != "x" ||
		!reflect.DeepEqual(out.Cuisines, []string{"chinese"}) {
		t.Errorf("explicit values must win: %+v", out)
	}
}

func TestMergeIntent_BestAndNearestTogetherSortsByDistance(t *testing.T) {
	votes := 50
	out, err := mergeIntent(repository.ListFilters{Near: &repository.GeoFilter{Lat: 1, Lng: 1}}, nlq.Intent{Best: true, Nearest: true, MinVotes: &votes})
	if err != nil {
		t.Fatal(err)
	}
	if out.Sort != repository.SortDistance || out.Desc {
		t.Errorf("sort: %+v", out)
	}
	if out.MinVotes == nil || *out.MinVotes != 50 {
		t.Errorf("best should still apply the vote floor: %+v", out)
	}
}

func TestMergeIntent_ExplicitSortAndVotesAreKept(t *testing.T) {
	explicitVotes, intentVotes := 5, 50
	out, err := mergeIntent(
		repository.ListFilters{Sort: repository.SortName, MinVotes: &explicitVotes, Near: &repository.GeoFilter{}},
		nlq.Intent{Best: true, Nearest: true, MinVotes: &intentVotes},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Sort != repository.SortName || *out.MinVotes != 5 {
		t.Errorf("explicit sort/votes overridden: %+v", out)
	}
}

func TestMergeIntent_NearestWithoutLocationIsInvalid(t *testing.T) {
	if _, err := mergeIntent(repository.ListFilters{}, nlq.Intent{Nearest: true}); !errors.Is(err, repository.ErrInvalidFilter) {
		t.Fatalf("err = %v", err)
	}
}

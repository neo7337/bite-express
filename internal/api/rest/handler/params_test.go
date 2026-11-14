package handler

import (
	"errors"
	"net/url"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
)

func parse(t *testing.T, qs string) (repository.ListFilters, error) {
	t.Helper()
	q, err := url.ParseQuery(qs)
	if err != nil {
		t.Fatal(err)
	}
	return parseListFilters(q)
}

func TestParseListFilters_Empty(t *testing.T) {
	f, err := parse(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Near != nil || f.MinRating != nil || f.MaxCost != nil || f.Limit != 0 {
		t.Fatalf("expected zero filters, got %+v", f)
	}
}

func TestParseListFilters_Full(t *testing.T) {
	f, err := parse(t, "name=pizza&city=Gurgaon&cuisine=north indian,chinese&cuisine=Cafe&minRating=4.2&maxCost=1000&lat=28.45&lng=77.02&radius=2000&sort=cost&order=desc&limit=10&offset=5")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "pizza" || f.City != "Gurgaon" {
		t.Errorf("text filters: %+v", f)
	}
	if len(f.Cuisines) != 3 {
		t.Errorf("cuisines: %v", f.Cuisines)
	}
	if f.MinRating == nil || *f.MinRating != 4.2 || f.MaxCost == nil || *f.MaxCost != 1000 {
		t.Errorf("bounds: %+v", f)
	}
	if f.Near == nil || f.Near.Lat != 28.45 || f.Near.Lng != 77.02 || f.Near.RadiusMeters != 2000 {
		t.Errorf("near: %+v", f.Near)
	}
	if f.Sort != repository.SortCost || !f.Desc || f.Limit != 10 || f.Offset != 5 {
		t.Errorf("sort/page: %+v", f)
	}
}

func TestParseListFilters_OrderDefaults(t *testing.T) {
	f, _ := parse(t, "sort=rating")
	if !f.Desc {
		t.Error("rating should default to desc")
	}
	f, _ = parse(t, "sort=name")
	if f.Desc {
		t.Error("name should default to asc")
	}
	f, _ = parse(t, "sort=rating&order=asc")
	if f.Desc {
		t.Error("explicit asc must win")
	}
}

func TestParseListFilters_Errors(t *testing.T) {
	for _, qs := range []string{
		"minRating=abc", "maxCost=1.5", "maxCost=-1", "lat=28", "lng=77", "radius=100",
		"lat=95&lng=77", "lat=NaN&lng=77", "lat=1&lng=1&radius=-5",
		"sort=popularity", "order=up", "limit=0", "limit=x", "offset=-1",
	} {
		if _, err := parse(t, qs); !errors.Is(err, errBadParam) {
			t.Errorf("%q: expected errBadParam, got %v", qs, err)
		}
	}
}

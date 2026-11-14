package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/internal/search/nlq"
)

func doSearch(t *testing.T, svc *fakeSvc, rawQuery string) (int, string) {
	t.Helper()
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	status, body := NewRestaurantsHandler(svc).search(context.Background(), q)
	b, _ := json.Marshal(body)
	return status, string(b)
}

func TestSearch_NoQueryIsPassedThroughAsEmpty(t *testing.T) {
	svc := &fakeSvc{}
	status, _ := doSearch(t, svc, "city=Gurgaon&limit=5")
	if status != http.StatusOK || svc.searchCalls != 1 || svc.searchedQ != "" {
		t.Fatalf("status=%d calls=%d q=%q", status, svc.searchCalls, svc.searchedQ)
	}
	if svc.searchedFilters.City != "Gurgaon" || svc.searchedFilters.Limit != 5 {
		t.Errorf("explicit filters not forwarded: %+v", svc.searchedFilters)
	}
}

func TestSearch_QueryIsTrimmedAndForwarded(t *testing.T) {
	svc := &fakeSvc{}
	doSearch(t, svc, "q="+url.QueryEscape("  best   cafes in gurgaon "))
	if svc.searchedQ != "best cafes in gurgaon" {
		t.Errorf("q = %q", svc.searchedQ)
	}
}

func TestSearch_ExplicitParamsAndQueryTogether(t *testing.T) {
	svc := &fakeSvc{}
	status, _ := doSearch(t, svc, "q=cheap+cafes&maxCost=300&lat=28.4&lng=77.0&sort=rating")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	f := svc.searchedFilters
	if svc.searchedQ != "cheap cafes" || f.MaxCost == nil || *f.MaxCost != 300 || f.Near == nil || f.Sort != repository.SortRating {
		t.Errorf("q=%q filters=%+v", svc.searchedQ, f)
	}
}

func TestSearch_RejectsBadInputWithoutCallingTheService(t *testing.T) {
	for name, qs := range map[string]string{
		"query too long":   "q=" + strings.Repeat("a", maxQueryLen+1),
		"bad limit":        "q=cafes&limit=0",
		"lat without lng":  "q=cafes&lat=28.4",
		"unknown sort":     "q=cafes&sort=popularity",
		"non numeric cost": "q=cafes&maxCost=cheap",
	} {
		t.Run(name, func(t *testing.T) {
			svc := &fakeSvc{}
			status, body := doSearch(t, svc, qs)
			if status != http.StatusBadRequest || svc.searchCalls != 0 {
				t.Errorf("status=%d calls=%d body=%s", status, svc.searchCalls, body)
			}
		})
	}
}

func TestSearch_ServiceErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"invalid filter", repository.ErrInvalidFilter, http.StatusBadRequest},
		{"wrapped invalid filter", fmt.Errorf("%w: a nearest query needs lat and lng", repository.ErrInvalidFilter), http.StatusBadRequest},
		{"unexpected", errors.New("mongo: connection refused at 10.0.0.5"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := doSearch(t, &fakeSvc{err: c.err}, "q=nearest+cafe")
			if status != c.want {
				t.Fatalf("status = %d, want %d", status, c.want)
			}
			if c.want == http.StatusInternalServerError && strings.Contains(body, "10.0.0.5") {
				t.Errorf("internal error leaked: %s", body)
			}
		})
	}
}

func TestSearch_ResponseIncludesInterpretation(t *testing.T) {
	cost := 1000
	svc := &fakeSvc{intent: nlq.Intent{City: "gurgaon", Cuisines: []string{"south indian"}, MaxCost: &cost, Unmatched: []string{"cheap"}}}
	status, body := doSearch(t, svc, "q=anything")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	for _, want := range []string{`"items":[]`, `"hasMore":false`, `"interpreted":{`, `"city":"gurgaon"`, `"maxCost":1000`, `"unmatchedTerms":["cheap"]`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in %s", want, body)
		}
	}
}

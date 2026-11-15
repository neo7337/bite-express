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
	"github.com/neo7337/bite-express/pkg"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func doList(t *testing.T, svc *fakeSvc, rawQuery string) (int, string) {
	t.Helper()
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	status, body := NewRestaurantsHandler(svc).list(context.Background(), q)
	b, _ := json.Marshal(body)
	return status, string(b)
}

func TestList_ForwardsParsedFilters(t *testing.T) {
	svc := &fakeSvc{}
	status, _ := doList(t, svc, "city=Gurgaon&cuisine=south+indian&minRating=4&limit=5&sort=rating")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	f := svc.listedFilters
	if f.City != "Gurgaon" || f.Limit != 5 || f.Sort != repository.SortRating {
		t.Errorf("filters not forwarded: %+v", f)
	}
	if f.MinRating == nil || *f.MinRating != 4 {
		t.Errorf("minRating = %v, want 4", f.MinRating)
	}
}

func TestList_BadParamsNeverReachTheService(t *testing.T) {
	for _, q := range []string{"lat=28.4", "limit=abc", "sort=bogus", "minRating=high"} {
		t.Run(q, func(t *testing.T) {
			svc := &fakeSvc{}
			status, _ := doList(t, svc, q)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", status)
			}
			if svc.listedFilters.City != "" || svc.listedFilters.Limit != 0 {
				t.Errorf("service was called with %+v", svc.listedFilters)
			}
		})
	}
}

func TestList_ServiceErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"invalid filter", fmt.Errorf("sort: %w", repository.ErrInvalidFilter), http.StatusBadRequest},
		{"unexpected", errors.New("mongo: connection refused at 10.0.0.5"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := doList(t, &fakeSvc{listErr: c.err}, "")
			if status != c.want {
				t.Fatalf("status = %d, want %d", status, c.want)
			}
			if c.want == http.StatusInternalServerError && strings.Contains(body, "10.0.0.5") {
				t.Errorf("internal error leaked: %s", body)
			}
		})
	}
}

func TestList_ResponseShape(t *testing.T) {
	rating, dist := 4.5, 120.0
	svc := &fakeSvc{listResult: repository.ListResult{
		HasMore: true,
		Items: []repository.ListItem{{
			Restaurant: pkg.Restaurant{
				ID: bson.NewObjectID(), TenantID: "secret-tenant", Name: "Spice",
				Address:  pkg.Address{Line: "12 Main", City: "Gurgaon", Country: "India"},
				Cuisines: []string{"North Indian"}, Rating: pkg.Rating{Average: &rating},
			},
			DistanceMeters: &dist,
		}},
	}}
	status, body := doList(t, svc, "lat=28.4&lng=77.0")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	for _, want := range []string{`"hasMore":true`, `"name":"Spice"`, `"city":"Gurgaon"`, `"country":"India"`, `"averageRating":4.5`, `"distance":120`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "secret-tenant") {
		t.Errorf("tenant id leaked: %s", body)
	}
}

func TestList_NoResultsIsAnEmptyArray(t *testing.T) {
	_, body := doList(t, &fakeSvc{}, "")
	if !strings.Contains(body, `"items":[]`) {
		t.Errorf("want an empty array, got %s", body)
	}
}

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/internal/search/nlq"
	"github.com/neo7337/bite-express/pkg"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeSvc implements restaurantService. All receivers are pointers so that
// *fakeSvc satisfies the interface and captured state is shared.
type fakeSvc struct {
	r       *pkg.Restaurant
	err     error
	created *pkg.Restaurant // captured by CreateRestaurant

	// list
	listedFilters repository.ListFilters
	listResult    repository.ListResult
	listErr       error

	// search
	searchCalls     int
	searchedQ       string
	searchedFilters repository.ListFilters
	result          repository.ListResult
	intent          nlq.Intent
}

func (f *fakeSvc) GetRestaurants(_ context.Context, lf repository.ListFilters) (repository.ListResult, error) {
	f.listedFilters = lf
	return f.listResult, f.listErr
}

func (f *fakeSvc) GetRestaurantById(context.Context, string) (*pkg.Restaurant, error) {
	return f.r, f.err
}

func (f *fakeSvc) CreateRestaurant(_ context.Context, r *pkg.Restaurant) (*pkg.Restaurant, error) {
	f.created = r
	if f.err != nil {
		return nil, f.err
	}
	r.ID = bson.NewObjectID()
	return r, nil
}

func (f *fakeSvc) SearchRestaurants(_ context.Context, q string, lf repository.ListFilters) (repository.ListResult, nlq.Intent, error) {
	f.searchCalls++
	f.searchedQ, f.searchedFilters = q, lf
	return f.result, f.intent, f.err
}

func TestFindByID_StatusMapping(t *testing.T) {
	cases := []struct {
		name string
		svc  *fakeSvc
		want int
	}{
		{"found", &fakeSvc{r: &pkg.Restaurant{Name: "Spice"}}, http.StatusOK},
		{"invalid id", &fakeSvc{err: repository.ErrInvalidId}, http.StatusBadRequest},
		{"wrapped invalid id", &fakeSvc{err: fmt.Errorf("lookup: %w", repository.ErrInvalidId)}, http.StatusBadRequest},
		{"not found", &fakeSvc{err: repository.ErrNotFound}, http.StatusNotFound},
		{"nil without error", &fakeSvc{}, http.StatusNotFound},
		{"unexpected error", &fakeSvc{err: errors.New("mongo down")}, http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewRestaurantsHandler(c.svc)
			got, body := h.findByID(context.Background(), "abc")
			if got != c.want {
				t.Fatalf("status = %d, want %d (body %v)", got, c.want, body)
			}
		})
	}
}

func TestFindByID_DoesNotLeakInternalErrors(t *testing.T) {
	h := NewRestaurantsHandler(&fakeSvc{err: errors.New("mongo: connection refused at 10.0.0.5")})
	_, body := h.findByID(context.Background(), "abc")
	b, _ := json.Marshal(body)
	if strings.Contains(string(b), "10.0.0.5") {
		t.Fatalf("internal error leaked: %s", b)
	}
}

func TestFindByID_ResponseBody(t *testing.T) {
	id := bson.NewObjectID()
	h := NewRestaurantsHandler(&fakeSvc{r: &pkg.Restaurant{ID: id, TenantID: "secret-tenant", Name: "Spice"}})
	_, body := h.findByID(context.Background(), id.Hex())
	b, _ := json.Marshal(body)
	if !strings.Contains(string(b), id.Hex()) || strings.Contains(string(b), "secret-tenant") {
		t.Fatalf("unexpected body: %s", b)
	}
}

const validBody = `{"name":"Spice","address":"12 Main","city":"Gurgaon","country":"India",
 "lat":28.45,"lng":77.02,"cuisines":["North Indian"," north  indian ","Chinese"]}`

func TestCreate_Success(t *testing.T) {
	svc := &fakeSvc{}
	status, body, id := NewRestaurantsHandler(svc).create(context.Background(), strings.NewReader(validBody))
	if status != http.StatusCreated || id == "" {
		t.Fatalf("status=%d id=%q body=%v", status, id, body)
	}
	c := svc.created
	if c == nil {
		t.Fatal("service was not called")
	}
	if len(c.Cuisines) != 2 || c.CuisineKeys[0] != "north indian" {
		t.Errorf("cuisines not normalised: %v / %v", c.Cuisines, c.CuisineKeys)
	}
	if c.Location == nil || c.Location.Coordinates[0] != 77.02 || c.Location.Coordinates[1] != 28.45 { // [lng, lat]
		t.Errorf("location: %+v", c.Location)
	}
	if c.TenantID != "" { // tenant is the service's job, never the client's
		t.Errorf("handler must not set tenant, got %q", c.TenantID)
	}
	if c.Pricing.Currency != "INR" || c.Source.Dataset != "api" {
		t.Errorf("defaults: %+v", c)
	}
	b, _ := json.Marshal(body)
	if strings.Contains(strings.ToLower(string(b)), "tenant") || !strings.Contains(string(b), id) {
		t.Errorf("unexpected body: %s", b)
	}
}

func TestCreate_Rejects(t *testing.T) {
	cases := map[string]struct {
		body string
		want int
	}{
		"malformed json":  {`{`, http.StatusBadRequest},
		"empty body":      {``, http.StatusBadRequest},
		"unknown field":   {`{"name":"x","tenantId":"evil"}`, http.StatusBadRequest},
		"client sets id":  {`{"id":"abc","name":"x"}`, http.StatusBadRequest},
		"trailing data":   {validBody + `{}`, http.StatusBadRequest},
		"missing fields":  {`{}`, http.StatusBadRequest},
		"lat without lng": {`{"name":"a","address":"b","city":"c","country":"d","lat":1,"cuisines":["x"]}`, http.StatusBadRequest},
		"too large":       {`{"name":"` + strings.Repeat("a", 2<<20) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &fakeSvc{}
			body := http.MaxBytesReader(nil, io.NopCloser(strings.NewReader(c.body)), maxBodyBytes)
			status, _, id := NewRestaurantsHandler(svc).create(context.Background(), body)
			if status != c.want {
				t.Errorf("status=%d, want %d", status, c.want)
			}
			if svc.created != nil || id != "" {
				t.Error("service must not be called for a rejected request")
			}
		})
	}
}

func TestCreate_ValidationDetails(t *testing.T) {
	status, body, _ := NewRestaurantsHandler(&fakeSvc{}).create(context.Background(), strings.NewReader(`{}`))
	m, ok := body.(map[string]any)
	if status != http.StatusBadRequest || !ok {
		t.Fatalf("status=%d body=%T", status, body)
	}
	details, _ := m["details"].([]string)
	if len(details) < 4 {
		t.Errorf("expected per-field details, got %v", details)
	}
}

func TestCreate_ServiceErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"conflict", repository.ErrConflict, http.StatusConflict},
		{"wrapped conflict", fmt.Errorf("x: %w", repository.ErrConflict), http.StatusConflict},
		{"unexpected", errors.New("mongo: connection refused at 10.0.0.5"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body, id := NewRestaurantsHandler(&fakeSvc{err: c.err}).create(context.Background(), strings.NewReader(validBody))
			if status != c.want || id != "" {
				t.Fatalf("status=%d id=%q, want %d", status, id, c.want)
			}
			if b, _ := json.Marshal(body); strings.Contains(string(b), "10.0.0.5") {
				t.Errorf("internal error leaked: %s", b)
			}
		})
	}
}

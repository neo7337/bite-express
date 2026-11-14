package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type fakeSvc struct {
	r   *pkg.Restaurant
	err error
}

func (f fakeSvc) GetRestaurants(context.Context, repository.ListFilters) (repository.ListResult, error) {
	return repository.ListResult{}, nil
}

func (f fakeSvc) GetRestaurantById(context.Context, string) (*pkg.Restaurant, error) {
	return f.r, f.err
}

func TestFindByID_StatusMapping(t *testing.T) {
	cases := []struct {
		name string
		svc  fakeSvc
		want int
	}{
		{"found", fakeSvc{r: &pkg.Restaurant{Name: "Spice"}}, http.StatusOK},
		{"invalid id", fakeSvc{err: repository.ErrInvalidId}, http.StatusBadRequest},
		{"wrapped invalid id", fakeSvc{err: fmt.Errorf("lookup: %w", repository.ErrInvalidId)}, http.StatusBadRequest},
		{"not found", fakeSvc{err: repository.ErrNotFound}, http.StatusNotFound},
		{"nil without error", fakeSvc{}, http.StatusNotFound},
		{"unexpected error", fakeSvc{err: errors.New("mongo down")}, http.StatusInternalServerError},
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
	h := NewRestaurantsHandler(fakeSvc{err: errors.New("mongo: connection refused at 10.0.0.5")})
	_, body := h.findByID(context.Background(), "abc")
	b, _ := json.Marshal(body)
	if strings.Contains(string(b), "10.0.0.5") {
		t.Fatalf("internal error leaked: %s", b)
	}
}

func TestFindByID_ResponseBody(t *testing.T) {
	id := bson.NewObjectID()
	h := NewRestaurantsHandler(fakeSvc{r: &pkg.Restaurant{ID: id, TenantID: "secret-tenant", Name: "Spice"}})
	_, body := h.findByID(context.Background(), id.Hex())
	b, _ := json.Marshal(body)
	if !strings.Contains(string(b), id.Hex()) || strings.Contains(string(b), "secret-tenant") {
		t.Fatalf("unexpected body: %s", b)
	}
}

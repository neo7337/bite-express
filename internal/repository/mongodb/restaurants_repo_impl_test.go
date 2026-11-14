package mongodb

import (
	"context"
	"errors"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
)

// The id is validated before the database is touched, so no Mongo is needed here.
func TestFindRestaurantById_InvalidID(t *testing.T) {
	repo := &MongoDBRestaurantsRepo{}
	for _, id := range []string{"", "abc", "not-a-hex-object-id!!!!!", "12345678901234567890123"} {
		r, err := repo.FindRestaurantById(context.Background(), "t", id)
		if !errors.Is(err, repository.ErrInvalidId) || r != nil {
			t.Errorf("id %q: got (%v, %v), want ErrInvalidId", id, r, err)
		}
	}
}

// A nil restaurant is rejected before the database is touched, so no Mongo is needed.
func TestInsert_NilRestaurant(t *testing.T) {
	r, err := (&MongoDBRestaurantsRepo{}).Insert(context.Background(), nil)
	if err == nil || r != nil {
		t.Fatalf("got (%v, %v), want an error", r, err)
	}
}

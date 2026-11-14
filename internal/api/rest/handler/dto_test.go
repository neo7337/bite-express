package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestToListResponse(t *testing.T) {
	avg, dist := 4.5, 123.0
	id := bson.NewObjectID()
	res := toListResponse(repository.ListResult{HasMore: true, Items: []repository.ListItem{{
		Restaurant: pkg.Restaurant{
			ID: id, TenantID: "secret", Name: "Spice",
			Address:  pkg.Address{Line: "12 Main", City: "Gurgaon", Country: "India"},
			Cuisines: []string{"Indian"}, Rating: pkg.Rating{Average: &avg},
		},
		DistanceMeters: &dist,
	}}})
	b, _ := json.Marshal(res)
	s := string(b)
	for _, want := range []string{`"id":"` + id.Hex() + `"`, `"averageRating":4.5`, `"distance":123`, `"city":"Gurgaon"`, `"hasMore":true`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	if strings.Contains(s, "secret") || strings.Contains(strings.ToLower(s), "tenant") {
		t.Errorf("internal field leaked: %s", s)
	}
}

func TestToListResponse_EmptyIsArray(t *testing.T) {
	b, _ := json.Marshal(toListResponse(repository.ListResult{}))
	if string(b) != `{"items":[],"hasMore":false}` {
		t.Fatalf("got %s", b)
	}
}

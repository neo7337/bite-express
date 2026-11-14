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

func TestToDetailDTO(t *testing.T) {
	avg, cost := 4.1, 800
	id := bson.NewObjectID()
	d := toDetailDTO(&pkg.Restaurant{
		ID: id, TenantID: "secret", SourceID: 99, Name: "Spice",
		Address:    pkg.Address{Line: "12 Main", Locality: "Sector 29", City: "Gurgaon", CountryCode: 1, Country: "India"},
		Location:   &pkg.GeoPoint{Type: "Point", Coordinates: []float64{77.02, 28.45}}, // [lng, lat]
		Cuisines:   []string{"Indian"},
		Pricing:    pkg.Pricing{CostForTwo: &cost, Currency: "INR", PriceRange: 3},
		Rating:     pkg.Rating{Average: &avg, Votes: 120, Text: "Very Good"},
		Flags:      pkg.Flags{HasOnlineDelivery: true},
		Attributes: map[string]any{"x": 1},
	})
	if d.ID != id.Hex() || d.Locality != "Sector 29" || d.CountryCode != 1 {
		t.Errorf("basic fields: %+v", d)
	}
	if d.Location == nil || d.Location.Lat != 28.45 || d.Location.Lng != 77.02 {
		t.Errorf("lat/lng swapped or missing: %+v", d.Location)
	}
	if d.Pricing.CostForTwo == nil || *d.Pricing.CostForTwo != 800 || d.Rating.Votes != 120 || !d.Flags.HasOnlineDelivery {
		t.Errorf("nested: %+v", d)
	}
	b, _ := json.Marshal(d)
	s := strings.ToLower(string(b))
	for _, leaked := range []string{"secret", "tenant", "sourceid", "attributes"} {
		if strings.Contains(s, leaked) {
			t.Errorf("%q leaked: %s", leaked, b)
		}
	}
}

func TestToDetailDTO_Sparse(t *testing.T) {
	d := toDetailDTO(&pkg.Restaurant{Name: "No Data"})
	if d.Location != nil || d.Pricing.CostForTwo != nil || d.Rating.Average != nil {
		t.Errorf("expected nils: %+v", d)
	}
	if d.Cuisines == nil {
		t.Error("cuisines should be [] not null")
	}
	for _, bad := range [][]float64{nil, {1}, {1, 2, 3}} {
		if toDetailDTO(&pkg.Restaurant{Location: &pkg.GeoPoint{Coordinates: bad}}).Location != nil {
			t.Errorf("coordinates %v should not produce a location", bad)
		}
	}
}

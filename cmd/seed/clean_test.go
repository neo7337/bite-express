package main

import (
	"reflect"
	"testing"
)

func baseRow() map[string]string {
	return map[string]string{
		"Restaurant ID": "1001", "Restaurant Name": "Test Kitchen", "Country Code": "1",
		"City": "Gurgaon", "Address": "12 MG Road", "Locality": "Sector 29",
		"Longitude": "77.0266", "Latitude": "28.4595",
		"Cuisines": "South Indian, Chinese", "Average Cost for two": "800",
		"Currency": "Indian Rupees(Rs.)", "Has Table booking": "Yes",
		"Has Online delivery": "No", "Is delivering now": "No", "Price range": "2",
		"Aggregate rating": "4.3", "Rating text": "Very Good", "Votes": "120",
	}
}

func TestCleanRow(t *testing.T) {
	t.Run("valid row", func(t *testing.T) {
		r, reason := cleanRow(baseRow())
		if reason != "" {
			t.Fatalf("unexpected skip: %s", reason)
		}
		if r.SourceID != 1001 || r.Address.Country != "India" || r.Pricing.Currency != "INR" {
			t.Errorf("unexpected doc: %+v", r)
		}
		if r.Location == nil || !reflect.DeepEqual(r.Location.Coordinates, []float64{77.0266, 28.4595}) {
			t.Errorf("location must be [lng, lat], got %+v", r.Location)
		}
		if r.Rating.Average == nil || *r.Rating.Average != 4.3 {
			t.Errorf("rating: %+v", r.Rating)
		}
	})

	skips := map[string]func(map[string]string){
		"invalid id":   func(m map[string]string) { m["Restaurant ID"] = "abc" },
		"missing name": func(m map[string]string) { m["Restaurant Name"] = " " },
	}
	for want, mutate := range skips {
		row := baseRow()
		mutate(row)
		if _, reason := cleanRow(row); reason != want {
			t.Errorf("want skip %q, got %q", want, reason)
		}
	}

	t.Run("bad coordinates leave no location", func(t *testing.T) {
		for _, c := range [][2]string{{"0", "0"}, {"", ""}, {"77", "95.5"}, {"east", "28"}} {
			row := baseRow()
			row["Longitude"], row["Latitude"] = c[0], c[1]
			r, _ := cleanRow(row)
			if r.Location != nil {
				t.Errorf("coordinates %v should be dropped, got %+v", c, r.Location)
			}
		}
	})

	t.Run("cuisines are trimmed and deduplicated", func(t *testing.T) {
		row := baseRow()
		row["Cuisines"] = "North Indian,  Chinese ,north indian, "
		r, _ := cleanRow(row)
		if !reflect.DeepEqual(r.Cuisines, []string{"North Indian", "Chinese"}) ||
			!reflect.DeepEqual(r.CuisineKeys, []string{"north indian", "chinese"}) {
			t.Errorf("got %v / %v", r.Cuisines, r.CuisineKeys)
		}
		row["Cuisines"] = ""
		r, _ = cleanRow(row)
		if r.Cuisines == nil || len(r.Cuisines) != 0 {
			t.Error("blank cuisines must be an empty slice, not nil")
		}
	})

	t.Run("zero cost is unknown, not free", func(t *testing.T) {
		row := baseRow()
		row["Average Cost for two"] = "0"
		if r, _ := cleanRow(row); r.Pricing.CostForTwo != nil {
			t.Errorf("cost should be nil, got %d", *r.Pricing.CostForTwo)
		}
	})

	t.Run("unrated and out of range ratings are null", func(t *testing.T) {
		row := baseRow()
		row["Aggregate rating"], row["Rating text"] = "0", "Not rated"
		if r, _ := cleanRow(row); r.Rating.Average != nil {
			t.Error("Not rated should be null")
		}
		row = baseRow()
		row["Aggregate rating"] = "5.7"
		if r, _ := cleanRow(row); r.Rating.Average != nil {
			t.Error("5.7 should be null")
		}
	})

}

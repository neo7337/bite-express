package handler

import (
	"fmt"
	"strings"
	"testing"
)

func validRequest() CreateRestaurantRequest {
	return CreateRestaurantRequest{Name: "n", Address: "a", City: "c", Country: "India", Cuisines: []string{"x"}}
}

func TestValidate_ValidRequest(t *testing.T) {
	if errs := validRequest().validate(); len(errs) != 0 {
		t.Fatalf("valid request rejected: %v", errs)
	}
	r := validRequest()
	lat, lng, cost := 28.4, 77.0, 0
	r.Lat, r.Lng = &lat, &lng
	r.Pricing.CostForTwo = &cost // zero cost is allowed
	r.Pricing.PriceRange = 4
	r.Pricing.Currency = "INR"
	if errs := r.validate(); len(errs) != 0 {
		t.Fatalf("valid request with options rejected: %v", errs)
	}
}

func TestValidate_Rejects(t *testing.T) {
	lat, lng := 10.0, 10.0
	badLat, badLng := 91.0, 181.0
	neg := -1
	cases := map[string]func(*CreateRestaurantRequest){
		"blank name":       func(r *CreateRestaurantRequest) { r.Name = "  " },
		"long name":        func(r *CreateRestaurantRequest) { r.Name = strings.Repeat("a", maxName+1) },
		"missing address":  func(r *CreateRestaurantRequest) { r.Address = "" },
		"missing city":     func(r *CreateRestaurantRequest) { r.City = "" },
		"missing country":  func(r *CreateRestaurantRequest) { r.Country = "" },
		"no cuisines":      func(r *CreateRestaurantRequest) { r.Cuisines = []string{" ", ""} },
		"nil cuisines":     func(r *CreateRestaurantRequest) { r.Cuisines = nil },
		"too many cuisine": func(r *CreateRestaurantRequest) { r.Cuisines = numbered(maxCuisines + 1) },
		"long cuisine":     func(r *CreateRestaurantRequest) { r.Cuisines = []string{strings.Repeat("a", maxCuisineLen+1)} },
		"lat only":         func(r *CreateRestaurantRequest) { r.Lat = &lat },
		"lng only":         func(r *CreateRestaurantRequest) { r.Lng = &lng },
		"lat out of range": func(r *CreateRestaurantRequest) { r.Lat, r.Lng = &badLat, &lng },
		"lng out of range": func(r *CreateRestaurantRequest) { r.Lat, r.Lng = &lat, &badLng },
		"price range high": func(r *CreateRestaurantRequest) { r.Pricing.PriceRange = 5 },
		"price range neg":  func(r *CreateRestaurantRequest) { r.Pricing.PriceRange = -1 },
		"negative cost":    func(r *CreateRestaurantRequest) { r.Pricing.CostForTwo = &neg },
		"bad currency":     func(r *CreateRestaurantRequest) { r.Pricing.Currency = "RUPEES" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRequest()
			mutate(&r)
			if len(r.validate()) == 0 {
				t.Error("expected a validation error")
			}
		})
	}
}

func TestValidate_ReportsEveryProblem(t *testing.T) {
	if n := len((CreateRestaurantRequest{}).validate()); n < 5 {
		t.Errorf("want one message per missing field, got %d", n)
	}
}

func TestValidate_CuisineLimitCountsDistinctNames(t *testing.T) {
	r := validRequest()
	r.Cuisines = []string{"Cafe", "cafe", " CAFE "} // all the same cuisine
	if errs := r.validate(); len(errs) != 0 {
		t.Errorf("duplicates should collapse, got %v", errs)
	}
}

func numbered(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprint("cuisine", i)
	}
	return out
}

package handler

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/neo7337/bite-express/pkg"
)

const (
	maxName, maxAddress, maxCity = 200, 500, 100
	maxCuisines, maxCuisineLen   = 20, 50
)

// validate returns one message per problem; empty means valid.
func (r CreateRestaurantRequest) validate() []string {
	var errs []string
	required := func(field, v string, max int) {
		v = strings.TrimSpace(v)
		switch {
		case v == "":
			errs = append(errs, field+" is required")
		case utf8.RuneCountInString(v) > max:
			errs = append(errs, fmt.Sprintf("%s must be at most %d characters", field, max))
		}
	}
	required("name", r.Name, maxName)
	required("address", r.Address, maxAddress)
	required("city", r.City, maxCity)
	required("country", r.Country, maxCity)

	switch {
	case (r.Lat == nil) != (r.Lng == nil):
		errs = append(errs, "lat and lng must be given together")
	case r.Lat != nil:
		if bad(*r.Lat) || bad(*r.Lng) || *r.Lat < -90 || *r.Lat > 90 || *r.Lng < -180 || *r.Lng > 180 {
			errs = append(errs, "lat/lng out of range")
		}
	}

	names, _ := pkgNormalizeCuisines(r.Cuisines)
	switch {
	case len(names) == 0:
		errs = append(errs, "cuisines must contain at least one cuisine")
	case len(names) > maxCuisines:
		errs = append(errs, fmt.Sprintf("cuisines must have at most %d entries", maxCuisines))
	}
	for _, c := range names {
		if utf8.RuneCountInString(c) > maxCuisineLen {
			errs = append(errs, fmt.Sprintf("cuisine %q is longer than %d characters", c, maxCuisineLen))
			break
		}
	}

	if c := r.Pricing.CostForTwo; c != nil && *c < 0 {
		errs = append(errs, "pricing.costForTwo must not be negative")
	}
	if p := r.Pricing.PriceRange; p < 0 || p > 4 {
		errs = append(errs, "pricing.priceRange must be between 1 and 4 (or omitted)")
	}
	if cur := strings.TrimSpace(r.Pricing.Currency); cur != "" && len(cur) != 3 {
		errs = append(errs, "pricing.currency must be a 3-letter code")
	}
	return errs
}

func bad(f float64) bool { return math.IsNaN(f) || math.IsInf(f, 0) }

var pkgNormalizeCuisines = pkg.NormalizeCuisines

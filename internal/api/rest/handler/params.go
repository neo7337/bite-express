package handler

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/neo7337/bite-express/internal/repository"
)

// errBadParam marks a client error in the query string (mapped to HTTP 400).
var errBadParam = errors.New("bad query parameter")

func badParam(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errBadParam, fmt.Sprintf(format, args...))
}

var sortFields = map[string]repository.SortField{
	"rating":   repository.SortRating,
	"distance": repository.SortDistance,
	"name":     repository.SortName,
	"cost":     repository.SortCost,
}

func parseFloat(q url.Values, key string) (*float64, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, badParam("%s must be a number", key)
	}
	return &v, nil
}

func parseInt(q url.Values, key string) (*int, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, badParam("%s must be an integer", key)
	}
	return &v, nil
}

// parseListFilters turns the query string of GET /v1/restaurants into repository filters.
//
//	name, city          text filters
//	cuisine             repeatable and/or comma separated; a restaurant must serve all of them
//	minRating, maxCost  numeric bounds (maxCost is cost for two, in INR unless currency is set)
//	currency            currency code for cost filters
//	lat, lng, radius    location (radius in metres, optional); both lat and lng are required
//	sort                rating | distance | name | cost
//	order               asc | desc (rating defaults to desc, everything else to asc)
//	limit, offset       pagination
func parseListFilters(q url.Values) (repository.ListFilters, error) {
	f := repository.ListFilters{
		Name:     strings.TrimSpace(q.Get("name")),
		City:     strings.TrimSpace(q.Get("city")),
		Currency: strings.ToUpper(strings.TrimSpace(q.Get("currency"))),
	}
	for _, v := range q["cuisine"] {
		f.Cuisines = append(f.Cuisines, strings.Split(v, ",")...)
	}

	var err error
	if f.MinRating, err = parseFloat(q, "minRating"); err != nil {
		return f, err
	}
	if f.MaxCost, err = parseInt(q, "maxCost"); err != nil {
		return f, err
	}
	if f.MaxCost != nil && *f.MaxCost < 0 {
		return f, badParam("maxCost must not be negative")
	}

	lat, err := parseFloat(q, "lat")
	if err != nil {
		return f, err
	}
	lng, err := parseFloat(q, "lng")
	if err != nil {
		return f, err
	}
	radius, err := parseFloat(q, "radius")
	if err != nil {
		return f, err
	}
	switch {
	case lat != nil && lng != nil:
		if *lat < -90 || *lat > 90 || *lng < -180 || *lng > 180 {
			return f, badParam("lat/lng out of range")
		}
		f.Near = &repository.GeoFilter{Lat: *lat, Lng: *lng}
		if radius != nil {
			if *radius < 0 {
				return f, badParam("radius must not be negative")
			}
			f.Near.RadiusMeters = *radius
		}
	case lat != nil || lng != nil:
		return f, badParam("lat and lng must be given together")
	case radius != nil:
		return f, badParam("radius needs lat and lng")
	}

	if s := strings.ToLower(strings.TrimSpace(q.Get("sort"))); s != "" {
		sf, ok := sortFields[s]
		if !ok {
			return f, badParam("sort must be one of rating, distance, name, cost")
		}
		f.Sort = sf
	}
	switch order := strings.ToLower(strings.TrimSpace(q.Get("order"))); order {
	case "desc":
		f.Desc = true
	case "asc":
	case "":
		f.Desc = f.Sort == repository.SortRating
	default:
		return f, badParam("order must be asc or desc")
	}

	limit, err := parseInt(q, "limit")
	if err != nil {
		return f, err
	}
	if limit != nil {
		if *limit < 1 {
			return f, badParam("limit must be at least 1")
		}
		f.Limit = *limit // the repository caps it
	}
	offset, err := parseInt(q, "offset")
	if err != nil {
		return f, err
	}
	if offset != nil {
		if *offset < 0 {
			return f, badParam("offset must not be negative")
		}
		f.Offset = *offset
	}
	return f, nil
}

const maxQueryLen = 200

// parseQuery trims and collapses whitespace; empty means "no natural language query".
func parseQuery(raw string) (string, error) {
	q := strings.Join(strings.Fields(raw), " ")
	if utf8.RuneCountInString(q) > maxQueryLen {
		return "", badParam("q must be at most %d characters", maxQueryLen)
	}
	return q, nil
}

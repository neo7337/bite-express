package mongodb

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/neo7337/bite-express/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	defaultLimit    = 20
	maxLimit        = 100
	defaultCurrency = "INR"
)

func normalizeFilters(f repository.ListFilters) repository.ListFilters {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	if f.Limit > maxLimit {
		f.Limit = maxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	if f.Sort == "" {
		if f.Near != nil {
			f.Sort = repository.SortDistance
		} else {
			f.Sort = repository.SortRating
			f.Desc = true
		}
	}

	if f.Currency == "" && (f.MaxCost != nil || f.Sort == repository.SortCost) {
		f.Currency = defaultCurrency
	}

	cuisines := make([]string, 0, len(f.Cuisines))
	for _, c := range f.Cuisines {
		if c = strings.ToLower(strings.TrimSpace(c)); c != "" {
			cuisines = append(cuisines, c)
		}
	}
	f.Cuisines = cuisines
	f.Name = strings.TrimSpace(f.Name)
	f.City = strings.TrimSpace(f.City)
	f.Text = strings.TrimSpace(f.Text)
	return f
}

func constainsRegex(s string) bson.Regex {
	return bson.Regex{Pattern: regexp.QuoteMeta(s), Options: "i"}
}

func buildMatch(f repository.ListFilters) bson.D {
	match := bson.D{{Key: "tenantId", Value: f.TenantID}}
	if f.Name != "" {
		match = append(match, bson.E{Key: "name", Value: constainsRegex(f.Name)})
	}
	if f.City != "" {
		city := bson.Regex{Pattern: "^" + regexp.QuoteMeta(f.City) + "$", Options: "i"}
		match = append(match, bson.E{Key: "address.city", Value: city})
	}

	if len(f.Cuisines) > 0 {
		match = append(match, bson.E{Key: "cuisineKeys", Value: bson.D{{Key: "$all", Value: f.Cuisines}}})
	}

	switch {
	case f.MinRating != nil:
		match = append(match, bson.E{Key: "rating.average", Value: bson.D{{Key: "$gte", Value: *f.MinRating}}})
	case f.Sort == repository.SortRating && !f.Desc:
		match = append(match, bson.E{Key: "rating.average", Value: bson.D{{Key: "$gte", Value: 0}}})
	}

	cost := bson.D{}
	if f.MaxCost != nil {
		cost = append(cost, bson.E{Key: "$lte", Value: *f.MaxCost})
	}
	if f.Sort == repository.SortCost {
		cost = append(cost, bson.E{Key: "$gt", Value: 0})
	}
	if len(cost) > 0 {
		match = append(cost, bson.E{Key: "pricing.costForTwo", Value: cost})
	}
	if f.Currency != "" {
		match = append(match, bson.E{Key: "pricing.currency", Value: f.Currency})
	}

	if f.Text != "" {
		match = append(match, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "name", Value: constainsRegex(f.Text)}},
			bson.D{{Key: "cuisineKeys", Value: constainsRegex(f.Text)}},
		}})
	}
	return match
}

func buildSort(f repository.ListFilters) bson.D {
	dir := 1
	if f.Desc {
		dir = -1
	}

	switch f.Sort {
	case repository.SortDistance:
		return bson.D{{Key: "distance", Value: dir}, {Key: "_id", Value: 1}}
	case repository.SortName:
		return bson.D{{Key: "name", Value: dir}, {Key: "_id", Value: 1}}
	case repository.SortCost:
		return bson.D{{Key: "pricing.costForTwo", Value: dir}, {Key: "_id", Value: 1}}
	default: // rating: more votes first among equal ratings
		return bson.D{{Key: "rating.average", Value: dir}, {Key: "rating.votes", Value: -1}, {Key: "_id", Value: 1}}
	}
}

func buildListPipeline(f repository.ListFilters) ([]bson.D, error) {
	f = normalizeFilters(f)

	switch f.Sort {
	case repository.SortRating, repository.SortDistance, repository.SortName, repository.SortCost:
	default:
		return nil, fmt.Errorf("%w: unknown sort %q", repository.ErrInvalidFilter, f.Sort)
	}

	if f.Sort == repository.SortDistance && f.Near == nil {
		return nil, fmt.Errorf("%w: sorting by distance needs a location", repository.ErrInvalidFilter)
	}
	if n := f.Near; n != nil {
		if n.Lat < -90 || n.Lat > 90 || n.Lng < -180 || n.Lng > 180 {
			return nil, fmt.Errorf("%w: location out of range", repository.ErrInvalidFilter)
		}
		if n.RadiusMeters < 0 {
			return nil, fmt.Errorf("%w: radius must not be negative", repository.ErrInvalidFilter)
		}
	}

	match := buildMatch(f)
	var pipeline []bson.D

	if f.Near != nil {
		// $geonear must be the first stage
		geo := bson.D{
			{Key: "near", Value: bson.D{
				{Key: "type", Value: "Point"},
				{Key: "coordinates", Value: bson.A{f.Near.Lng, f.Near.Lat}},
			}},
			{Key: "distanceField", Value: "distance"},
			{Key: "spherical", Value: true},
			{Key: "query", Value: match},
		}
		if f.Near.RadiusMeters > 0 {
			geo = append(geo, bson.E{Key: "maxDistance", Value: f.Near.RadiusMeters})
		}
		pipeline = append(pipeline, bson.D{{Key: "$geoNear", Value: geo}})
	} else {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: match}})
	}

	pipeline = append(pipeline, bson.D{{Key: "$sort", Value: buildSort(f)}})
	if f.Offset > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$skip", Value: int64(f.Offset)}})
	}
	pipeline = append(pipeline, bson.D{{Key: "$limit", Value: int64(f.Limit + 1)}})
	return pipeline, nil
}

package mongodb

import (
	"errors"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func field(d bson.D, key string) (any, bool) {
	for _, e := range d {
		if e.Key == key {
			return e.Value, true
		}
	}
	return nil, false
}

func TestBuildMatch_AlwaysScopedToTenant(t *testing.T) {
	cost := 500
	rating := 4.0
	cases := map[string]repository.ListFilters{
		"empty":    {TenantID: "t1"},
		"maxCost":  {TenantID: "t1", MaxCost: &cost, Currency: "INR"},
		"sortCost": {TenantID: "t1", Sort: repository.SortCost},
		"all":      {TenantID: "t1", Name: "x", City: "Gurgaon", Cuisines: []string{"cafe"}, MinRating: &rating, MaxCost: &cost, Text: "vada"},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			v, ok := field(buildMatch(normalizeFilters(f)), "tenantId")
			if !ok || v != "t1" {
				t.Fatalf("tenantId missing or wrong: %v", v)
			}
		})
	}
}

func TestBuildMatch_CostFilter(t *testing.T) {
	cost := 500
	m := buildMatch(normalizeFilters(repository.ListFilters{TenantID: "t", MaxCost: &cost}))
	v, ok := field(m, "pricing.costForTwo")
	if !ok {
		t.Fatal("cost filter missing")
	}
	if lte, _ := field(v.(bson.D), "$lte"); lte != 500 {
		t.Errorf("$lte = %v", lte)
	}
	if cur, _ := field(m, "pricing.currency"); cur != defaultCurrency {
		t.Errorf("currency should default to %s, got %v", defaultCurrency, cur)
	}
}

func TestBuildMatch_SortCostExcludesMissingCost(t *testing.T) {
	m := buildMatch(normalizeFilters(repository.ListFilters{TenantID: "t", Sort: repository.SortCost}))
	v, _ := field(m, "pricing.costForTwo")
	if gt, _ := field(v.(bson.D), "$gt"); gt != 0 {
		t.Errorf("$gt = %v", gt)
	}
}

func TestBuildMatch_Cuisines(t *testing.T) {
	m := buildMatch(normalizeFilters(repository.ListFilters{TenantID: "t", Cuisines: []string{" North Indian ", "", "CAFE"}}))
	v, ok := field(m, "cuisineKeys")
	if !ok {
		t.Fatal("cuisineKeys filter missing")
	}
	all, _ := field(v.(bson.D), "$all")
	got := all.([]string)
	if len(got) != 2 || got[0] != "north indian" || got[1] != "cafe" {
		t.Errorf("cuisines not normalised: %v", got)
	}
}

func TestBuildMatch_CityIsAnchoredAndEscaped(t *testing.T) {
	m := buildMatch(normalizeFilters(repository.ListFilters{TenantID: "t", City: "a.b"}))
	v, _ := field(m, "address.city")
	re := v.(bson.Regex)
	if re.Pattern != `^a\.b$` || re.Options != "i" {
		t.Errorf("regex = %+v", re)
	}
}

func TestBuildMatch_Rating(t *testing.T) {
	min := 4.0
	m := buildMatch(normalizeFilters(repository.ListFilters{TenantID: "t", MinRating: &min}))
	v, _ := field(m, "rating.average")
	if gte, _ := field(v.(bson.D), "$gte"); gte != 4.0 {
		t.Errorf("$gte = %v", gte)
	}
	// ascending rating sort must skip unrated restaurants
	m = buildMatch(normalizeFilters(repository.ListFilters{TenantID: "t", Sort: repository.SortRating}))
	if _, ok := field(m, "rating.average"); !ok {
		t.Error("asc rating sort should exclude null ratings")
	}
	// default (desc) must not
	m = buildMatch(normalizeFilters(repository.ListFilters{TenantID: "t"}))
	if _, ok := field(m, "rating.average"); ok {
		t.Error("default listing should not filter on rating")
	}
}

func TestBuildMatch_TextSearch(t *testing.T) {
	m := buildMatch(normalizeFilters(repository.ListFilters{TenantID: "t", Text: "vada pav"}))
	v, ok := field(m, "$or")
	if !ok || len(v.(bson.A)) != 2 {
		t.Fatalf("$or = %v", v)
	}
}

func TestBuildListPipeline_Defaults(t *testing.T) {
	p, err := buildListPipeline(repository.ListFilters{TenantID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 3 {
		t.Fatalf("want $match,$sort,$limit got %d stages", len(p))
	}
	if p[0][0].Key != "$match" || p[1][0].Key != "$sort" || p[2][0].Key != "$limit" {
		t.Errorf("stages: %v", p)
	}
	if p[2][0].Value != int64(defaultLimit+1) {
		t.Errorf("limit = %v, want default+1 for hasMore detection", p[2][0].Value)
	}
	sort := p[1][0].Value.(bson.D)
	if sort[0].Key != "rating.average" || sort[0].Value != -1 {
		t.Errorf("default sort: %v", sort)
	}
}

func TestBuildListPipeline_LimitCappedAndSkip(t *testing.T) {
	p, err := buildListPipeline(repository.ListFilters{TenantID: "t", Limit: 10_000, Offset: 40})
	if err != nil {
		t.Fatal(err)
	}
	if p[len(p)-1][0].Value != int64(maxLimit+1) {
		t.Errorf("limit not capped: %v", p[len(p)-1][0].Value)
	}
	if p[2][0].Key != "$skip" || p[2][0].Value != int64(40) {
		t.Errorf("skip stage: %v", p[2])
	}
}

func TestBuildListPipeline_GeoNearIsFirstStage(t *testing.T) {
	p, err := buildListPipeline(repository.ListFilters{
		TenantID: "t", City: "Gurgaon",
		Near: &repository.GeoFilter{Lat: 28.4, Lng: 77.0, RadiusMeters: 1500},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p[0][0].Key != "$geoNear" {
		t.Fatalf("first stage = %s", p[0][0].Key)
	}
	geo := p[0][0].Value.(bson.D)
	near, _ := field(geo, "near")
	coords, _ := field(near.(bson.D), "coordinates")
	c := coords.(bson.A)
	if c[0] != 77.0 || c[1] != 28.4 { // GeoJSON order is [lng, lat]
		t.Errorf("coordinates = %v", c)
	}
	if md, _ := field(geo, "maxDistance"); md != 1500.0 {
		t.Errorf("maxDistance = %v", md)
	}
	q, _ := field(geo, "query")
	if tid, _ := field(q.(bson.D), "tenantId"); tid != "t" {
		t.Error("tenant filter must be inside $geoNear.query")
	}
	// distance becomes the default sort when a location is given
	sort := p[1][0].Value.(bson.D)
	if sort[0].Key != "distance" {
		t.Errorf("sort = %v", sort)
	}
}

func TestBuildListPipeline_NoMaxDistanceWithoutRadius(t *testing.T) {
	p, _ := buildListPipeline(repository.ListFilters{TenantID: "t", Near: &repository.GeoFilter{Lat: 1, Lng: 1}})
	if _, ok := field(p[0][0].Value.(bson.D), "maxDistance"); ok {
		t.Error("maxDistance should be omitted when radius is 0")
	}
}

func TestBuildListPipeline_InvalidFilters(t *testing.T) {
	cases := map[string]repository.ListFilters{
		"unknown sort":     {TenantID: "t", Sort: "popularity"},
		"distance no loc":  {TenantID: "t", Sort: repository.SortDistance},
		"lat out of range": {TenantID: "t", Near: &repository.GeoFilter{Lat: 91}},
		"lng out of range": {TenantID: "t", Near: &repository.GeoFilter{Lng: -181}},
		"negative radius":  {TenantID: "t", Near: &repository.GeoFilter{RadiusMeters: -1}},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := buildListPipeline(f); !errors.Is(err, repository.ErrInvalidFilter) {
				t.Errorf("want ErrInvalidFilter, got %v", err)
			}
		})
	}
}

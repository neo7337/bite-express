package main

import (
	"strconv"
	"strings"

	"github.com/neo7337/bite-express/pkg"
)

var countries = map[int]string{
	1: "India", 14: "Australia", 30: "Brazil", 37: "Canada", 94: "Indonesia",
	148: "New Zealand", 162: "Philippines", 166: "Qatar", 184: "Singapore",
	189: "South Africa", 191: "Sri Lanka", 208: "Turkey",
	214: "United Arab Emirates", 215: "United Kingdom", 216: "United States",
}

func cleanRow(row map[string]string) (pkg.Restaurant, string) {
	get := func(key string) string { return strings.TrimSpace(row[key]) }

	id, err := strconv.ParseInt(get("Restaurant ID"), 10, 64)
	if err != nil || id <= 0 {
		return pkg.Restaurant{}, "invalid id"
	}

	name := get("Restaurant Name")
	if name == "" {
		return pkg.Restaurant{}, "missing name"
	}

	code, _ := strconv.Atoi(get("Country Code"))
	r := pkg.Restaurant{
		SourceID: id,
		Name:     name,
		Address: pkg.Address{
			Line:        get("Address"),
			Locality:    get("Locality"),
			City:        get("City"),
			CountryCode: code,
			Country:     countries[code],
		},
		CuisineKeys: []string{},
		Cuisines:    []string{},
		Source:      pkg.SourceInfo{Dataset: "zomato-kaggle"},
	}

	lng, errLng := strconv.ParseFloat(get("Longitude"), 64)
	lat, errLat := strconv.ParseFloat(get("Latitude"), 64)
	valid := errLng == nil && errLat == nil && lng >= -180 && lng <= 180 && lat >= -90 && lat <= 90 && !(lng == 0 && lat == 0)
	if valid {
		r.Location = &pkg.GeoPoint{Type: "Point", Coordinates: []float64{lng, lat}}
	}

	seen := map[string]bool{}
	for _, c := range strings.Split(get("Cuisines"), ",") {
		c = strings.Join(strings.Fields(c), " ")
		key := strings.ToLower(c)
		if c == "" || seen[key] {
			continue
		}
		seen[key] = true
		r.Cuisines = append(r.Cuisines, c)
		r.CuisineKeys = append(r.CuisineKeys, key)
	}

	if n, err := strconv.Atoi(get("Average Cost for two")); err == nil && n > 0 {
		r.Pricing.CostForTwo = &n
	}

	r.Pricing.Currency = get("Currency")
	if r.Pricing.Currency == "Indian Rupees(Rs.)" {
		r.Pricing.Currency = "INR"
	}
	r.Pricing.PriceRange, _ = strconv.Atoi(get("Price Range"))
	r.Rating.Text = get("Rating text")
	r.Rating.Votes, _ = strconv.Atoi(get("Votes"))
	if f, err := strconv.ParseFloat(get("Aggregate rating"), 64); err == nil &&
		f >= 0 && f <= 5 && !strings.EqualFold(r.Rating.Text, "Not rated") {
		r.Rating.Average = &f
	}

	yes := func(key string) bool { return strings.EqualFold(get(key), "yes") }
	r.Flags = pkg.Flags{
		HasTableBooking:   yes("Has Table booking"),
		HasOnlineDelivery: yes("Has Online delivery"),
		IsDeliveringNow:   yes("Is delivering now"),
	}
	return r, ""
}

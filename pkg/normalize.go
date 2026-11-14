package pkg

import "strings"

func NormalizeCuisines(raw []string) (names, keys []string) {
	names, keys = []string{}, []string{}
	seen := map[string]bool{}
	for _, c := range raw {
		c = strings.Join(strings.Fields(c), " ")
		key := strings.ToLower(c)
		if c == "" || seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, c)
		keys = append(keys, key)
	}
	return names, keys
}

func NormalizeCurrency(c string) string {
	c = strings.TrimSpace(c)
	if c == "Indian Rupees(Rs.)" {
		return "INR"
	}
	return strings.ToUpper(c)
}

func NewGeoPoint(lat, lng float64) *GeoPoint {
	return &GeoPoint{Type: "Point", Coordinates: []float64{lng, lat}}
}

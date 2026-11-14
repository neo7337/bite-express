package handler

import (
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
)

// RestaurantDTO is the public shape of a restaurant in list responses.
type RestaurantDTO struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Address       string   `json:"address"`
	City          string   `json:"city"`
	Country       string   `json:"country"`
	Cuisines      []string `json:"cuisines"`
	AverageRating *float64 `json:"averageRating"`
	Distance      *float64 `json:"distance,omitempty"` // metres, only when a location was given
}

type ListResponse struct {
	Items   []RestaurantDTO `json:"items"`
	HasMore bool            `json:"hasMore"`
}

func toListResponse(r repository.ListResult) ListResponse {
	items := make([]RestaurantDTO, 0, len(r.Items))
	for _, it := range r.Items {
		cuisines := it.Cuisines
		if cuisines == nil {
			cuisines = []string{}
		}
		items = append(items, RestaurantDTO{
			ID:            it.ID.Hex(),
			Name:          it.Name,
			Address:       it.Address.Line,
			City:          it.Address.City,
			Country:       it.Address.Country,
			Cuisines:      cuisines,
			AverageRating: it.Rating.Average,
			Distance:      it.DistanceMeters,
		})
	}
	return ListResponse{Items: items, HasMore: r.HasMore}
}

type LocationDTO struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type PricingDTO struct {
	CostForTwo *int   `json:"costForTwo"`
	Currency   string `json:"currency"`
	PriceRange int    `json:"priceRange"`
}

type RatingDTO struct {
	Average *float64 `json:"average"`
	Votes   int      `json:"votes"`
	Text    string   `json:"text"`
}

type FlagsDTO struct {
	HasTableBooking   bool `json:"hasTableBooking"`
	HasOnlineDelivery bool `json:"hasOnlineDelivery"`
	IsDeliveringNow   bool `json:"isDeliveringNow"`
}

// RestaurantDetailDTO is the public shape of GET /v1/restaurants/{id}.
type RestaurantDetailDTO struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Address     string       `json:"address"`
	Locality    string       `json:"locality"`
	City        string       `json:"city"`
	Country     string       `json:"country"`
	CountryCode int          `json:"countryCode"`
	Location    *LocationDTO `json:"location,omitempty"`
	Cuisines    []string     `json:"cuisines"`
	Pricing     PricingDTO   `json:"pricing"`
	Rating      RatingDTO    `json:"rating"`
	Flags       FlagsDTO     `json:"flags"`
}

func toDetailDTO(r *pkg.Restaurant) RestaurantDetailDTO {
	cuisines := r.Cuisines
	if cuisines == nil {
		cuisines = []string{}
	}
	d := RestaurantDetailDTO{
		ID:          r.ID.Hex(),
		Name:        r.Name,
		Address:     r.Address.Line,
		Locality:    r.Address.Locality,
		City:        r.Address.City,
		Country:     r.Address.Country,
		CountryCode: r.Address.CountryCode,
		Cuisines:    cuisines,
		Pricing:     PricingDTO{CostForTwo: r.Pricing.CostForTwo, Currency: r.Pricing.Currency, PriceRange: r.Pricing.PriceRange},
		Rating:      RatingDTO{Average: r.Rating.Average, Votes: r.Rating.Votes, Text: r.Rating.Text},
		Flags: FlagsDTO{
			HasTableBooking:   r.Flags.HasTableBooking,
			HasOnlineDelivery: r.Flags.HasOnlineDelivery,
			IsDeliveringNow:   r.Flags.IsDeliveringNow,
		},
	}
	// stored as GeoJSON: [longitude, latitude]
	if r.Location != nil && len(r.Location.Coordinates) == 2 {
		d.Location = &LocationDTO{Lng: r.Location.Coordinates[0], Lat: r.Location.Coordinates[1]}
	}
	return d
}

package handler

import "github.com/neo7337/bite-express/internal/repository"

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

package repository

import (
	"context"
	"errors"

	"github.com/neo7337/bite-express/pkg"
)

var (
	ErrNotFound      = errors.New("restaurant not found")
	ErrInvalidId     = errors.New("invalid restaurant id")
	ErrInvalidFilter = errors.New("invalid filter")
	ErrConflict      = errors.New("restaurant already exists")
)

type SortField string

const (
	SortRating   SortField = "rating"
	SortDistance SortField = "distance"
	SortName     SortField = "name"
	SortCost     SortField = "cost"
)

type GeoFilter struct {
	Lat, Lng     float64
	RadiusMeters float64
}

type ListFilters struct {
	TenantID  string
	Name      string
	City      string
	Cuisines  []string
	MinRating *float64
	MaxCost   *int
	Currency  string
	Text      string // free text from search, matched against name and cuisines
	Near      *GeoFilter
	Sort      SortField
	Desc      bool
	Limit     int
	Offset    int
}

type ListItem struct {
	pkg.Restaurant `bson:",inline"`
	DistanceMeters *float64 `bson:"distance,omitempty"`
}

type ListResult struct {
	Items   []ListItem
	HasMore bool
}

type RestaurantsRepository interface {
	Insert(ctx context.Context, r *pkg.Restaurant) (*pkg.Restaurant, error)
	FindRestaurantById(ctx context.Context, tenantID, id string) (*pkg.Restaurant, error)
	List(ctx context.Context, f ListFilters) (ListResult, error)
}

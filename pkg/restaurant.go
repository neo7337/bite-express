package pkg

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Restaurant struct {
	ID          bson.ObjectID  `bson:"_id,omitempty"`
	TenantID    string         `bson:"tenantId"`
	SourceID    int64          `bson:"sourceId,omitempty"`
	Name        string         `bson:"name"`
	Address     Address        `bson:"address"`
	Location    *GeoPoint      `bson:"location,omitempty"`
	Cuisines    []string       `bson:"cuisines"`
	CuisineKeys []string       `bson:"cuisineKeys"`
	Pricing     Pricing        `bson:"pricing"`
	Rating      Rating         `bson:"rating"`
	Flags       Flags          `bson:"flags"`
	Attributes  map[string]any `bson:"attributes"`
	Source      SourceInfo     `bson:"source"`
	CreatedAt   time.Time      `bson:"createdAt"`
	UpdatedAt   time.Time      `bson:"updatedAt"`
}

type Address struct {
	Line        string `bson:"line"`
	Locality    string `bson:"locality"`
	City        string `bson:"city"`
	CountryCode int    `bson:"countryCode"`
	Country     string `bson:"country"`
}

// Coordinates are GeoJSON order: [longitude, latitude]
type GeoPoint struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

type Pricing struct {
	CostForTwo *int   `bson:"costForTwo"`
	Currency   string `bson:"currency"`
	PriceRange int    `bson:"priceRange"`
}

type Rating struct {
	Average *float64 `bson:"average"`
	Votes   int      `bson:"votes"`
	Text    string   `bson:"text"`
}

type Flags struct {
	HasTableBooking   bool `bson:"hasTableBooking"`
	HasOnlineDelivery bool `bson:"hasOnlineDelivery"`
	IsDeliveringNow   bool `bson:"isDeliveringNow"`
}

type SourceInfo struct {
	Dataset string `bson:"dataset"`
}

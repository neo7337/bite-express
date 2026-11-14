package mongodb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const collectionRestaurants = "restaurants"

type MongoDBRestaurantsRepo struct {
	*MongoDBRepo
}

func createIndexes(ctx context.Context, coll *mongo.Collection) error {
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{"location", "2dsphere"}}},
		{
			Keys:    bson.D{{"tenantId", 1}, {"sourceId", 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"sourceId": bson.M{"$exists": true}}),
		},
		{Keys: bson.D{{"tenantId", 1}, {"address.city", 1}, {"rating.average", -1}}},
		{Keys: bson.D{{"tenantId", 1}, {"cuisineKeys", 1}, {"rating.average", -1}}},
	})
	return err
}

func (repo *MongoDBRepo) ensureIndexes() error {
	coll := repo.Database.Collection("restaurants")
	if err := createIndexes(context.Background(), coll); err != nil {
		log.Fatalf("error creating indexes: %v", err)
		return err
	}
	return nil
}

func NewMongoDBRestaurantsRepo(repo *MongoDBRepo) (*MongoDBRestaurantsRepo, error) {
	// a particular collection repo is opened, then we ensure the indexes for this collection
	err := repo.ensureIndexes()
	if err != nil {
		return nil, err
	}
	return &MongoDBRestaurantsRepo{
		MongoDBRepo: repo,
	}, nil
}

func (m *MongoDBRestaurantsRepo) List(ctx context.Context, f repository.ListFilters) (repository.ListResult, error) {
	f = normalizeFilters(f)

	pipeline, err := buildListPipeline(f)
	if err != nil {
		return repository.ListResult{}, err
	}

	cursor, err := m.Database.Collection(collectionRestaurants).Aggregate(ctx, pipeline)
	if err != nil {
		return repository.ListResult{}, fmt.Errorf("error listing restaurants: %w", err)
	}

	items := make([]repository.ListItem, 0, f.Limit+1)
	if err := cursor.All(ctx, &items); err != nil {
		return repository.ListResult{}, fmt.Errorf("error reading restaurants: %w", err)
	}

	hasMore := len(items) > f.Limit
	if hasMore {
		items = items[:f.Limit]
	}
	return repository.ListResult{Items: items, HasMore: hasMore}, nil
}

func (m *MongoDBRestaurantsRepo) FindRestaurantById(ctx context.Context, tenantID, id string) (*pkg.Restaurant, error) {
	parsedId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, repository.ErrInvalidId
	}
	filter := bson.M{
		"_id":      parsedId,
		"tenantId": tenantID,
	}
	var restaurant *pkg.Restaurant
	err = m.Database.Collection(collectionRestaurants).FindOne(ctx, filter).Decode(&restaurant)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return restaurant, nil
}

func (m *MongoDBRestaurantsRepo) Insert(ctx context.Context, r *pkg.Restaurant) (*pkg.Restaurant, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: nil restaurant", repository.ErrInvalidFilter)
	}

	now := time.Now().UTC()
	r.ID = bson.ObjectID{}
	r.CreatedAt, r.UpdatedAt = now, now

	res, err := m.Database.Collection(collectionRestaurants).InsertOne(ctx, r)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, repository.ErrConflict
		}
		return nil, fmt.Errorf("error inserting document: %w", err)
	}
	id, ok := res.InsertedID.(bson.ObjectID)
	if !ok {
		return nil, fmt.Errorf("unexpected inserted id type %T", res.InsertedID)
	}
	r.ID = id
	return r, nil
}

func (m *MongoDBRestaurantsRepo) Vocabulary(ctx context.Context, tenantID string) (repository.Vocabulary, error) {
	coll := m.Database.Collection(collectionRestaurants)
	filter := bson.D{{Key: "tenantId", Value: tenantID}}

	var cities, cuisines []string
	if err := coll.Distinct(ctx, "address.city", filter).Decode(&cities); err != nil {
		return repository.Vocabulary{}, fmt.Errorf("error reading cities: %w", err)
	}
	if err := coll.Distinct(ctx, "cuisineKeys", filter).Decode(&cuisines); err != nil {
		return repository.Vocabulary{}, fmt.Errorf("error reading cuisines: %w", err)
	}
	return repository.Vocabulary{Cities: cities, Cuisines: cuisines}, nil
}

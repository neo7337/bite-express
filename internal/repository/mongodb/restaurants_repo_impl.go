package mongodb

import (
	"context"
	"fmt"
	"log"

	"github.com/neo7337/bite-express/internal/repository"
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

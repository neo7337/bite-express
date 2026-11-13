package mongodb

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

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
		{Keys: bson.D{{"tenandId", 1}, {"address.city", 1}, {"rating.average", -1}}},
		{Keys: bson.D{{"tenandId", 1}, {"cuisineKeys", 1}, {"rating.average", -1}}},
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

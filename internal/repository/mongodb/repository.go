package mongodb

import (
	"context"
	"errors"

	"github.com/neo7337/bite-express/internal/config"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDBRepo struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewMongoDBRepo() *MongoDBRepo {
	return &MongoDBRepo{}
}

func (m *MongoDBRepo) Connect(connection config.Connection) (*MongoDBRepo, error) {
	credential := options.Credential{
		AuthSource: connection.AuthSource,
		Username:   connection.Username,
		Password:   connection.Password,
	}
	uri := connection.Uri
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetAuth(credential))
	if err != nil {
		logger.ErrorF("failed to connect to database: %v", err)
		return nil, err
	}
	m.Client = client
	dbName := connection.DbName
	db := client.Database(dbName)
	m.Database = db
	return m, nil
}

func (m *MongoDBRepo) Close() error {
	if m.Client != nil {
		err := m.Client.Disconnect(context.Background())
		if err != nil {
			return errors.New("Failed to close database connection: " + err.Error())
		}
		m.Client = nil
	}
	return nil
}

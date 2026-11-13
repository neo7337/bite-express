package connections

import (
	"errors"
	"io"

	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/repository"
	mongodb_repo "github.com/neo7337/bite-express/internal/repository/mongodb"
	"oss.nandlabs.io/golly/collections"
	"oss.nandlabs.io/golly/l3"
	"oss.nandlabs.io/golly/lifecycle"
)

var connectionClosers = collections.NewArrayList[io.Closer]()
var logger = l3.Get()

func GetRepoConnection(manager lifecycle.ComponentManager) *lifecycle.SimpleComponent {
	connectionManager := &lifecycle.SimpleComponent{
		CompId: "connection-manager",
		StartFunc: func() error {
			return OpenConnections()
		},
		AfterStart: func(err error) {
			if err != nil {
				logger.ErrorF("Failed to open connections", "error", err)
				manager.StopAll()
			} else {
				logger.Info("Connections opened successfully")
			}
		},
		StopFunc: func() error {
			return CloseConnections()
		},
	}
	return connectionManager
}

func OpenConnections() (err error) {
	configDetails := config.GetConfig()
	for _, provider := range configDetails.Repository.Providers {
		if !provider.Enabled {
			logger.InfoF("Skipping disabled provider", "name", provider.Name)
			continue
		}
		logger.Info("Opening connection for provider", "name", provider.Name)
		switch provider.Name {
		case "mongodb":
			conn, err := OpenMongoDbConnection(provider.Connection)
			if err != nil {
				logger.ErrorF("Failed to open a connection", "error", err)
				return err
			}
			return RegisterMongoRepos(conn, provider.Modules)
		}
	}
	return
}

func OpenMongoDbConnection(connection config.Connection) (db *mongodb_repo.MongoDBRepo, err error) {
	client := mongodb_repo.NewMongoDBRepo()
	cl, err := client.Connect(connection)
	if err != nil {
		err = errors.New("Failed to connect to Mongodb: " + err.Error())
		return
	}
	connectionClosers.Add(cl)
	logger.Info("MongoDB Connection Initialised")
	return
}

func RegisterMongoRepos(conn *mongodb_repo.MongoDBRepo, modules []string) error {
	for _, module := range modules {
		switch module {
		case "restaurants":
			repo, err := mongodb_repo.NewMongoDBRestaurantsRepo(conn)
			if err != nil {
				return err
			}
			repository.Manager.Register(repository.RestaurantsRepo, repo)
		}
	}
	return nil
}

func CloseConnections() (err error) {
	if !connectionClosers.IsEmpty() {
		for it := connectionClosers.Iterator(); it.HasNext(); {
			it.Next().Close()
		}
	}
	return
}

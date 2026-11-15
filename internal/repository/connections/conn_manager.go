package connections

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/internal/repository/cached"
	mongodb_repo "github.com/neo7337/bite-express/internal/repository/mongodb"
	"oss.nandlabs.io/golly/collections"
	"oss.nandlabs.io/golly/l3"
	"oss.nandlabs.io/golly/lifecycle"
)

var connectionClosers = collections.NewArrayList[io.Closer]()
var logger = l3.Get()

var (
	mongoConnMu sync.RWMutex
	mongoConn   *mongodb_repo.MongoDBRepo
)

func GetRepoConnection(manager lifecycle.ComponentManager) *lifecycle.SimpleComponent {
	connectionManager := &lifecycle.SimpleComponent{
		CompId: "connection-manager",
		StartFunc: func() error {
			return OpenConnections()
		},
		AfterStart: func(err error) {
			if err != nil {
				logger.ErrorF("Failed to open connections: %v", err)
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
			logger.InfoF("Skipping disabled provider: %v", provider.Name)
			continue
		}
		logger.Info("Opening connection for provider: %v", provider.Name)
		switch provider.Name {
		case "mongodb":
			conn, err := OpenMongoDbConnection(provider.Connection)
			if err != nil {
				logger.ErrorF("Failed to open a connection: %v", err)
				return err
			}
			return RegisterMongoRepos(conn, provider.Modules)
		}
	}
	return
}

func OpenMongoDbConnection(connection config.Connection) (db *mongodb_repo.MongoDBRepo, err error) {
	client := mongodb_repo.NewMongoDBRepo()
	db, err = client.Connect(connection)
	if err != nil {
		err = errors.New("Failed to connect to Mongodb: " + err.Error())
		return
	}
	connectionClosers.Add(db)
	mongoConnMu.Lock()
	mongoConn = db
	mongoConnMu.Unlock()
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
			repository.Manager.Register(repository.RestaurantsRepo, withCache(repo))
		}
	}
	return nil
}

func CloseConnections() (err error) {
	if !connectionClosers.IsEmpty() {
		for it := connectionClosers.Iterator(); it.HasNext(); {
			_ = it.Next().Close() // best effort on shutdown
		}
	}
	mongoConnMu.Lock()
	mongoConn = nil
	mongoConnMu.Unlock()
	return
}

const cacheStatsInterval = time.Minute

// withCache wraps the repository in the read cache when cache.enabled is true. Only the
// in-memory cache exists today; any other type is reported and ignored, so the service still
// starts, just without a cache.
func withCache(repo repository.RestaurantsRepository) repository.RestaurantsRepository {
	cfg := config.GetConfig()
	if cfg == nil || !cfg.Cache.Enabled {
		logger.Info("Read cache disabled")
		return repo
	}
	if t := cfg.Cache.Type; t != "" && t != "memory" {
		logger.ErrorF("cache.type %q is not supported yet (use \"memory\"), running without a cache", t)
		return repo
	}
	ttl := time.Duration(cfg.Cache.Config.Ttl) * time.Second
	c := cached.New(repo, cached.Options{TTL: ttl, MaxEntries: cfg.Cache.Config.MaxEntries})
	c.StartStatsLogger(cacheStatsInterval, logger.InfoF)
	logger.InfoF("Read cache enabled (in-memory, ttl=%v, maxEntries=%d)", ttl, cfg.Cache.Config.MaxEntries)
	return c
}

var ErrNoDatabase = errors.New("no database connection")

func PingDB(ctx context.Context) error {
	mongoConnMu.RLock()
	conn := mongoConn
	mongoConnMu.RUnlock()
	if conn == nil {
		return ErrNoDatabase
	}
	return conn.Ping(ctx)
}

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/neo7337/bite-express/internal/seed"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func main() {
	os.Exit(run())
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("invalid mongo uri: %w", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = client.Ping(pingCtx, readpref.Primary())
		cancel()
		if err == nil {
			return client, nil
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			_ = client.Disconnect(context.Background())
			return nil, fmt.Errorf("mongodb not reachable: %w", err)
		}
		log.Printf("waiting for mongodb: %v", err)
		time.Sleep(2 * time.Second)
	}
}

func run() int {
	var (
		file   = flag.String("file", env("SEED_FILE", "data/zomato.csv"), "path to the seed data csv")
		uri    = flag.String("mongo-uri", env("MONGO_URI", "mongodb://localhost:27017/?directConnection=true"), "MongoDB connection string")
		dbName = flag.String("db", env("MONGO_DB", "biteexpress"), "database name")
	)

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	raw, err := os.ReadFile(*file)
	if err != nil {
		log.Printf("cannot read %s: %v", *file, err)
		return 1
	}

	// seed reader

	var store seed.Store
	client, err := connect(ctx, *uri)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	db := client.Database(*dbName)
	// ensure indexes

	return 0
}

package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	var (
		file   = flag.String("file", env("SEED_FILE", "data/zomato.csv"), "path to the seed data csv")
		uri    = flag.String("mongo-uri", env("MONGO_URI", "mongodb://localhost:27017/?directConnection=true"), "MongoDB connection string")
		dbName = flag.String("db", env("MONGO_DB", "biteexpress"), "database name")
		tenant = flag.String("tenant", env("TENANT", "biteexpress"), "tenand id stored on every document")
	)

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rows, err := readRows(*file)
	if err != nil {
		log.Fatalf("cannot read %s: %v", *file, err)
	}

	client, err := connect(*uri)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	coll := client.Database(*dbName).Collection("restaurants")

	now := time.Now().UTC()
	seen := map[int64]bool{}
	skipped := map[string]int{}
	var models []mongo.WriteModel
	for _, row := range rows {
		doc, reason := cleanRow(row)
		if reason == "" && seen[doc.SourceID] {
			reason = "duplicate id"
		}
		if reason != "" {
			skipped[reason]++
			continue
		}
		seen[doc.SourceID] = true
		doc.TenantID = *tenant
		doc.CreatedAt, doc.UpdatedAt = now, now
		models = append(models, mongo.NewReplaceOneModel().
			SetFilter(bson.D{{"tenantId", doc.TenantID}, {"sourceID", doc.SourceID}}).
			SetReplacement(doc).
			SetUpsert(true))
	}

	res, err := coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		log.Fatalf("error writing: %v", err)
	}

	fmt.Printf("rows read: %d, loaded: %d (new: %d, existing: %d)\n", len(rows), len(models), res.UpsertedCount, res.MatchedCount)
	for reason, n := range skipped {
		fmt.Printf("skipped %d rows: %s\n", n, reason)
	}
}

func readRows(path string) ([]map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(raw) {
		text := make([]byte, 0, len(raw)+len(raw)/8)
		for _, b := range raw {
			text = utf8.AppendRune(text, rune(b))
		}
		raw = text
	}

	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		log.Fatalf("error reading records: %v", err)
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%s has no more data rows", path)
	}

	header := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := make(map[string]string, len(header))
		for i, h := range header {
			if i < len(rec) {
				row[h] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func connect(uri string) (*mongo.Client, error) {
	credential := options.Credential{
		AuthSource: "admin",
		Username:   "admin",
		Password:   "secretpassword",
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetAuth(credential))
	if err != nil {
		return nil, fmt.Errorf("invalid mongo uri: %w", err)
	}
	return client, err
}

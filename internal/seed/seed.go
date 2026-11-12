package seed

import (
	"context"

	"github.com/neo7337/bite-express/pkg"
)

const defaultBatchSize = 500

type Config struct {
	Tenant    string
	BatchSize int
	DryRun    bool
}

type Failure struct {
	SourceID string
	Err      string
}

type UpsertResult struct {
	Inserted  int
	Updated   int
	Unchanged int
	Failures  []Failure
}

type Store interface {
	Upsert(ctx context.Context, docs []pkg.Restaurant) (UpsertResult, error)
}

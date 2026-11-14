package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/internal/search/nlq"
	"github.com/neo7337/bite-express/pkg"
)

type RestaurantsService struct {
	repo   repository.RestaurantsRepository
	config *config.Config

	vocabMu sync.Mutex
	vocab   *nlq.Vocabulary // lazily loaded, see vocabulary()
}

func NewRestaurantsService(config *config.Config) *RestaurantsService {
	return &RestaurantsService{
		repo:   repository.Get[repository.RestaurantsRepository](repository.RestaurantsRepo),
		config: config,
	}
}

func (r *RestaurantsService) GetRestaurants(ctx context.Context, lf repository.ListFilters) (repository.ListResult, error) {
	if lf.TenantID == "" {
		lf.TenantID = r.config.System.TenantID
	}
	lr, err := r.repo.List(ctx, lf)
	return lr, err
}

func (r *RestaurantsService) GetRestaurantById(ctx context.Context, id string) (*pkg.Restaurant, error) {
	tenantId := r.config.System.TenantID
	restaurant, err := r.repo.FindRestaurantById(ctx, tenantId, id)
	return restaurant, err
}

func (r *RestaurantsService) CreateRestaurant(ctx context.Context, restaurant *pkg.Restaurant) (*pkg.Restaurant, error) {
	restaurant.TenantID = r.config.System.TenantID
	return r.repo.Insert(ctx, restaurant)
}

func (r *RestaurantsService) SearchRestaurants(ctx context.Context, q string, explicit repository.ListFilters) (repository.ListResult, nlq.Intent, error) {
	var intent nlq.Intent
	if q != "" {
		vocab, err := r.vocabulary(ctx)
		if err != nil {
			return repository.ListResult{}, intent, fmt.Errorf("loading search vocabulary: %w", err)
		}
		intent = nlq.Parse(q, vocab)
	}

	filters, err := mergeIntent(explicit, intent)
	if err != nil {
		return repository.ListResult{}, intent, err
	}
	result, err := r.GetRestaurants(ctx, filters)
	return result, intent, err
}

func (r *RestaurantsService) vocabulary(ctx context.Context) (nlq.Vocabulary, error) {
	r.vocabMu.Lock()
	defer r.vocabMu.Unlock()

	if r.vocab != nil {
		return *r.vocab, nil
	}

	v, err := r.repo.Vocabulary(ctx, r.config.System.TenantID)
	if err != nil {
		return nlq.Vocabulary{}, err
	}
	built := nlq.NewVocabulary(v.Cities, v.Cuisines)
	r.vocab = &built
	return built, nil
}

// mergeIntent combines the explicit query-string filters with what was understood from q.
// Explicit values win: the intent only fills fields the client left empty.
func mergeIntent(f repository.ListFilters, in nlq.Intent) (repository.ListFilters, error) {
	if f.City == "" {
		f.City = in.City
	}
	if len(f.Cuisines) == 0 {
		f.Cuisines = in.Cuisines
	}
	if f.MaxCost == nil {
		f.MaxCost = in.MaxCost
	}
	if f.Currency == "" {
		f.Currency = in.Currency
	}
	if f.MinRating == nil {
		f.MinRating = in.MinRating
	}
	if f.Text == "" {
		f.Text = in.Text
	}
	if in.Best && f.MinVotes == nil {
		f.MinVotes = in.MinVotes // "best" should not be won by a 5.0 from two votes
	}

	if in.Nearest && f.Near == nil {
		return f, fmt.Errorf("%w: a nearest query needs lat and lng", repository.ErrInvalidFilter)
	}
	if f.Sort == "" { // only choose a sort when the client did not
		switch {
		case in.Nearest:
			f.Sort = repository.SortDistance
		case in.Best:
			f.Sort, f.Desc = repository.SortRating, true
		}
	}
	return f, nil
}

package service

import (
	"context"

	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/repository"
)

type RestaurantsService struct {
	repo   repository.RestaurantsRepository
	config *config.Config
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

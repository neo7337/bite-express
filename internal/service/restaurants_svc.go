package service

import (
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

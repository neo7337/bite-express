package handler

import (
	"github.com/neo7337/bite-express/internal/service"
	"oss.nandlabs.io/golly/l3"
	"oss.nandlabs.io/golly/rest"
)

var logger = l3.Get

type RestaurantsHandler struct {
	svc *service.RestaurantsService
}

func NewRestaurantsHandler(svc *service.RestaurantsService) *RestaurantsHandler {
	return &RestaurantsHandler{
		svc: svc,
	}
}

func (h *RestaurantsHandler) RestaurantsHandler(ctx rest.ServerContext) {

}

package handler

import (
	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/internal/service"
	"github.com/neo7337/bite-express/pkg"
	"oss.nandlabs.io/golly/l3"
	"oss.nandlabs.io/golly/rest"
)

var logger = l3.Get()

type RestaurantsHandler struct {
	svc *service.RestaurantsService
}

func NewRestaurantsHandler(svc *service.RestaurantsService) *RestaurantsHandler {
	return &RestaurantsHandler{
		svc: svc,
	}
}

func (h *RestaurantsHandler) ListRestaurants(ctx rest.ServerContext) {
	list, err := h.svc.GetRestaurants(ctx.Context(), repository.ListFilters{})
	if err != nil {
		logger.ErrorF("Failed to list restaurants", "error", err)
		pkg.ResponseJSON(ctx, 500, map[string]string{"error": "Internal Server Error"})
		return
	}
	pkg.ResponseJSON(ctx, 200, list)
}

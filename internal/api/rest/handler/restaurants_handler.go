package handler

import (
	"errors"

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
	// NOTE: the only golly-specific call for query access; adjust here if the API differs.
	filters, err := parseListFilters(ctx.GetRequest().URL.Query())
	if err != nil {
		pkg.ResponseJSON(ctx, 400, map[string]string{"error": err.Error()})
		return
	}

	list, err := h.svc.GetRestaurants(ctx.Context(), filters)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidFilter) {
			pkg.ResponseJSON(ctx, 400, map[string]string{"error": err.Error()})
			return
		}
		logger.ErrorF("Failed to list restaurants", "error", err)
		pkg.ResponseJSON(ctx, 500, map[string]string{"error": "Internal Server Error"})
		return
	}
	pkg.ResponseJSON(ctx, 200, toListResponse(list))
}

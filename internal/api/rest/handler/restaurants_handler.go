package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
	"oss.nandlabs.io/golly/l3"
	"oss.nandlabs.io/golly/rest"
)

var logger = l3.Get()

// restaurantService is what the handler needs from the service layer
// (satisfied by *service.RestaurantsService; small so tests can fake it).
type restaurantService interface {
	GetRestaurants(ctx context.Context, lf repository.ListFilters) (repository.ListResult, error)
	GetRestaurantById(ctx context.Context, id string) (*pkg.Restaurant, error)
}

type RestaurantsHandler struct {
	svc restaurantService
}

func NewRestaurantsHandler(svc restaurantService) *RestaurantsHandler {
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

// FindRestaurantById serves GET /v1/restaurants/:id.
func (h *RestaurantsHandler) FindRestaurantById(ctx rest.ServerContext) {
	id, err := ctx.GetParam("id", rest.PathParam)
	if err != nil {
		pkg.ResponseJSON(ctx, http.StatusBadRequest, map[string]string{"error": "missing restaurant id"})
		return
	}
	status, body := h.findByID(ctx.Context(), id)
	pkg.ResponseJSON(ctx, status, body)
}

// findByID holds the endpoint logic without any HTTP framework types.
func (h *RestaurantsHandler) findByID(ctx context.Context, id string) (int, any) {
	r, err := h.svc.GetRestaurantById(ctx, id)
	switch {
	case errors.Is(err, repository.ErrInvalidId):
		return http.StatusBadRequest, map[string]string{"error": repository.ErrInvalidId.Error()}
	case errors.Is(err, repository.ErrNotFound):
		return http.StatusNotFound, map[string]string{"error": repository.ErrNotFound.Error()}
	case err != nil:
		logger.ErrorF("Failed to get restaurant", "error", err)
		return http.StatusInternalServerError, map[string]string{"error": "Internal Server Error"}
	case r == nil:
		return http.StatusNotFound, map[string]string{"error": repository.ErrNotFound.Error()}
	}
	return http.StatusOK, toDetailDTO(r)
}

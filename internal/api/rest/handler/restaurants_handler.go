package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/pkg"
	"oss.nandlabs.io/golly/l3"
	"oss.nandlabs.io/golly/rest"
)

var logger = l3.Get()

const maxBodyBytes = 1 << 20 // 1 MB

// restaurantService is what the handler needs from the service layer
// (satisfied by *service.RestaurantsService; small so tests can fake it).
type restaurantService interface {
	CreateRestaurant(ctx context.Context, r *pkg.Restaurant) (*pkg.Restaurant, error)
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

func (h *RestaurantsHandler) CreateRestaurant(ctx rest.ServerContext) {
	req := ctx.GetRequest()
	if ct := req.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		pkg.ResponseJSON(ctx, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type must be application/json"})
		return
	}

	body := http.MaxBytesReader(ctx.HttpResWriter(), req.Body, maxBodyBytes)
	status, resp, id := h.create(ctx.Context(), body)
	if id != "" {
		ctx.SetHeader("Location", strings.TrimSuffix(req.URL.Path, "/")+"/"+id)
	}
	pkg.ResponseJSON(ctx, status, resp)
}

func (h *RestaurantsHandler) create(ctx context.Context, body io.Reader) (int, any, string) {
	var in CreateRestaurantRequest
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"}, ""
		}
		return http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()}, ""
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return http.StatusBadRequest, map[string]string{"error": "body must contain a single JSON object"}, ""
	}
	if problems := in.validate(); len(problems) > 0 {
		return http.StatusBadRequest, map[string]any{"error": "validation failed", "details": problems}, ""
	}
	created, err := h.svc.CreateRestaurant(ctx, in.toRestaurant())
	switch {
	case errors.Is(err, repository.ErrConflict):
		return http.StatusConflict, map[string]string{"error": repository.ErrConflict.Error()}, ""
	case err != nil:
		logger.ErrorF("Failed to create restaurant", "error", err)
		return http.StatusInternalServerError, map[string]string{"error": "internal server error"}, ""
	}
	return http.StatusCreated, toDetailDTO(created), created.ID.Hex()
}

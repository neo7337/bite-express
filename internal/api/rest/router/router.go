package router

import (
	"context"
	"time"

	"github.com/neo7337/bite-express/internal/api/rest/handler"
	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/repository/connections"
	"github.com/neo7337/bite-express/internal/service"
	"github.com/neo7337/bite-express/pkg"
	"oss.nandlabs.io/golly/l3"
	"oss.nandlabs.io/golly/rest"
)

const readyTimeout = 2 * time.Second

var logger = l3.Get()
var pingDB = connections.PingDB

func RouterHandler(server rest.Server) rest.Server {
	server.Get("/healthz", func(ctx rest.ServerContext) {
		pkg.ResponseJSON(ctx, 200, map[string]string{"status": "ok"})
	})

	server.Get("/readyz", readyzHandler)

	conf := config.GetConfig()
	restaurantSvc := service.NewRestaurantsService(conf)
	restaurantHandler := handler.NewRestaurantsHandler(restaurantSvc)

	// register handlers
	server.Get("/v1/restaurants", restaurantHandler.ListRestaurants)
	// "search" must be registered before "/:id", otherwise it is matched as an id
	server.Get("/v1/restaurants/search", restaurantHandler.SearchRestaurants)
	server.Get("/v1/restaurants/:id", restaurantHandler.FindRestaurantById)
	server.Post("/v1/restaurants", restaurantHandler.CreateRestaurant)
	return server
}

func readyzHandler(ctx rest.ServerContext) {
	pingCtx, cancel := context.WithTimeout(context.Background(), readyTimeout)
	defer cancel()

	if err := pingDB(pingCtx); err != nil {
		logger.ErrorF("readiness check failed: %v", err)
		pkg.ResponseJSON(ctx, 503, map[string]string{"status": "unavailable", "mongo": "down"})
		return
	}
	pkg.ResponseJSON(ctx, 200, map[string]string{"status": "available", "mongo": "up"})
}

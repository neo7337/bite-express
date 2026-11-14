package router

import (
	"github.com/neo7337/bite-express/internal/api/rest/handler"
	"github.com/neo7337/bite-express/internal/config"
	"github.com/neo7337/bite-express/internal/service"
	"github.com/neo7337/bite-express/pkg"
	"oss.nandlabs.io/golly/rest"
)

func RouterHandler(server rest.Server) rest.Server {
	server.Get("/healthz", func(ctx rest.ServerContext) {
		pkg.ResponseJSON(ctx, 200, map[string]string{"status": "ok"})
	})

	conf := config.GetConfig()
	restaurantSvc := service.NewRestaurantsService(conf)
	restaurantHandler := handler.NewRestaurantsHandler(restaurantSvc)

	// register handlers
	server.Get("/v1/restaurants", restaurantHandler.ListRestaurants)
	server.Get("/v1/restaurants/:id", restaurantHandler.FindRestaurantById)
	return server
}

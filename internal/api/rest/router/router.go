package router

import (
	"github.com/neo7337/bite-express/pkg"
	"oss.nandlabs.io/golly/rest"
)

func RouterHandler(server rest.Server) rest.Server {
	server.Get("/healthz", func(ctx rest.ServerContext) {
		pkg.ResponseJSON(ctx, 200, map[string]string{"status": "ok"})
	})

	// register handlers

	return server
}

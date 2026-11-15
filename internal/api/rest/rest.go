package rest

import (
	"github.com/neo7337/bite-express/internal/api/rest/router"
	"github.com/neo7337/bite-express/internal/config"
	"oss.nandlabs.io/golly/lifecycle"
	"oss.nandlabs.io/golly/rest"
)

func GetRestServer(manager lifecycle.ComponentManager) lifecycle.Component {
	conf := config.GetConfig()
	serverOptions := rest.DefaultSrvOptions()
	serverOptions.PathPrefix = "/api"
	serverOptions.ListenHost = conf.System.Host
	serverOptions.ListenPort = int16(conf.System.Port)

	server, err := rest.NewServer(serverOptions)
	if err != nil {
		panic(err)
	}
	server.OnChange(func(prevState, newState lifecycle.ComponentState) {
		if prevState == lifecycle.Unknown && newState == lifecycle.Starting {
			server = router.RouterHandler(server)
		}
	})
	return server
}

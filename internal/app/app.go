package app

import (
	"github.com/neo7337/bite-express/internal/api/rest"
	"github.com/neo7337/bite-express/internal/repository/connections"
	"oss.nandlabs.io/golly/lifecycle"
)

var manager = lifecycle.NewSimpleComponentManager()

func Start() {
	conn := connections.GetRepoConnection(manager)
	manager.Register(conn)
	restServer := rest.GetRestServer(manager)
	manager.Register(restServer)
	manager.AddDependency(restServer.Id(), conn.Id())
	manager.StartAndWait()
}

func Shutdown() {
	manager.StopAll()
}

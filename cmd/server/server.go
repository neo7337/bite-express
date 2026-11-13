package main

import (
	"os"

	"github.com/neo7337/bite-express/internal/app"
	"github.com/neo7337/bite-express/internal/config"
	"oss.nandlabs.io/golly/l3"
)

var logger = l3.Get()

func main() {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "local"
	}
	logger.Info("Initiating biteexpress")
	configFileName := "config-" + appEnv + ".yml"
	// load the configuration file
	_, err := config.LoadConfig(configFileName)
	if err != nil {
		logger.ErrorF("Error loading config: ", err)
		return
	}
	app.Start()
}

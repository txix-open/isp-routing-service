package main

import (
	"github.com/integration-system/isp-kit/bootstrap"
	"github.com/integration-system/isp-kit/shutdown"
	"isp-routing-service/assembly"
	"isp-routing-service/conf"
)

var (
	version = "1.0.0"
)

func main() {
	boot := bootstrap.New(version, conf.Remote{}, nil)
	app := boot.App
	logger := app.Logger()

	assembly := assembly.New(boot)
	app.AddRunners(assembly.Runners()...)
	app.AddClosers(assembly.Closers()...)

	shutdown.On(func() {
		logger.Info(app.Context(), "starting shutdown")
		app.Shutdown()
		logger.Info(app.Context(), "shutdown completed")
	})

	err := app.Run()
	if err != nil {
		app.Shutdown()
		logger.Fatal(app.Context(), err)
	}
}

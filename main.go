package main

import (
	"isp-routing-service/assembly"
	"isp-routing-service/conf"

	"github.com/txix-open/isp-kit/bootstrap"
	"github.com/txix-open/isp-kit/cluster"
	"github.com/txix-open/isp-kit/shutdown"
)

var (
	version = "1.0.0"
)

func main() {
	boot := bootstrap.New(version, conf.Remote{}, nil, cluster.HttpTransport)
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
		boot.Fatal(err)
	}
}

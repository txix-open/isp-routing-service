package assembly

import (
	"context"

	"github.com/txix-open/isp-kit/app"
	"github.com/txix-open/isp-kit/bootstrap"
	"github.com/txix-open/isp-kit/cluster"
	"github.com/txix-open/isp-kit/http"

	"github.com/txix-open/isp-kit/log"
)

type Assembly struct {
	boot     *bootstrap.Bootstrap
	server   *http.Server
	proxyMap map[ProxyKey]Proxy
	logger   *log.Adapter
}

func New(boot *bootstrap.Bootstrap) *Assembly {
	return &Assembly{
		boot:     boot,
		server:   http.NewServer(boot.App.Logger()),
		proxyMap: make(map[ProxyKey]Proxy, 0),
		logger:   boot.App.Logger(),
	}
}

func (a *Assembly) ReceiveRoutes(_ context.Context, routingCfg cluster.RoutingConfig) error {
	locator := NewLocator(a.logger)
	locatorCfg := locator.LocatorConfig(routingCfg, a.proxyMap)
	a.server.Upgrade(locatorCfg.Handler)

	for key, oldProxy := range a.proxyMap {
		_, ok := locatorCfg.ProxyMap[key]
		if !ok {
			_ = oldProxy.Close()
		}
	}
	a.proxyMap = locatorCfg.ProxyMap

	return nil
}

func (a *Assembly) Runners() []app.Runner {
	eventHandler := cluster.NewEventHandler().
		RoutesReceiver(a)
	return []app.Runner{
		app.RunnerFunc(func(ctx context.Context) error {
			return a.server.ListenAndServe(a.boot.BindingAddress)
		}),
		app.RunnerFunc(func(ctx context.Context) error {
			return a.boot.ClusterCli.Run(ctx, eventHandler)
		}),
	}
}

func (a *Assembly) Closers() []app.Closer {
	return []app.Closer{
		a.boot.ClusterCli,
		app.CloserFunc(func() error {
			return a.server.Shutdown(context.Background())
		}),
		app.CloserFunc(func() error {
			for _, proxy := range a.proxyMap {
				_ = proxy.Close()
			}
			return nil
		}),
	}
}

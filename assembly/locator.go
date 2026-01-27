package assembly

import (
	"context"
	"fmt"
	"isp-routing-service/domain"
	"isp-routing-service/middleware"
	"isp-routing-service/service/proxy"
	"net"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	http2 "github.com/txix-open/isp-kit/http"
	"github.com/txix-open/isp-kit/http/endpoint"
	"github.com/txix-open/isp-kit/http/router"
	"github.com/txix-open/isp-kit/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/txix-open/isp-kit/cluster"
	"github.com/txix-open/isp-kit/grpc/client"
)

const (
	maxMessageSize = 64 * 1024 * 1024
	proxyTimeout   = 5 * time.Second
)

type Proxy interface {
	Handle(ctx *domain.RequestContext) error
	PrepareEndpoint(endpoint string, proxyApiPrefix bool) string
	Close() error
}

type Locator struct {
	logger log.Logger
}

func NewLocator(logger log.Logger) Locator {
	return Locator{
		logger: logger,
	}
}

type LocatorConfig struct {
	ProxyMap map[ProxyKey]Proxy
	Handler  http.Handler
}

type ProxyKey struct {
	Transport string
	Addresses string
}

func (l Locator) LocatorConfig(config cluster.RoutingConfig, oldProxyMap map[ProxyKey]Proxy) LocatorConfig {
	routes := l.getRoutes(config)

	groupMap := make(map[string][]domain.RouteKey)
	for route, cfg := range routes {
		addresses := append([]string{}, cfg.Addresses...)
		sort.Strings(addresses) // чтобы ключ был одинаковым для одинакового множества
		groupKey := strings.Join(addresses, ",")
		groupMap[groupKey] = append(groupMap[groupKey], route)
	}

	mux := router.New()
	mux.InternalRouter().NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, fmt.Sprintf("unknown endpoint '%s'", r.URL.Path), http.StatusNotImplemented)
	})

	middlewares := []http2.Middleware{
		middleware.ErrorHandler(l.logger),
		endpoint.RequestId(),
	}

	proxyMap := make(map[ProxyKey]Proxy, len(groupMap))
	for groupKey, routeKeys := range groupMap {
		addresses := strings.Split(groupKey, ",")
		// ОК, т.к. в getRoutes собираем только с endpoint'ами
		transport := routes[routeKeys[0]].Transport
		proxyKey := ProxyKey{
			Transport: transport,
			Addresses: groupKey,
		}

		var err error
		proxy, ok := oldProxyMap[proxyKey]
		if !ok {
			proxy, err = buildProxy(proxyKey.Transport, addresses)
			if err != nil {
				l.logger.Error(context.Background(), err)
				continue
			}
		}
		proxyMap[proxyKey] = proxy

		for _, routeKey := range routeKeys {
			l.registerEndpoint(mux, routeKey, proxy, middlewares)
		}
	}

	l.logger.Info(
		context.Background(),
		"change routing table",
		log.Int("totalBackends", len(proxyMap)),
		log.Int("totalEndpoints", len(routes)),
	)
	return LocatorConfig{
		ProxyMap: proxyMap,
		Handler:  mux,
	}
}

func (l Locator) getRoutes(config cluster.RoutingConfig) map[domain.RouteKey]*domain.RouteConfig {
	routes := make(map[domain.RouteKey]*domain.RouteConfig, len(config))

	for _, decl := range config {
		if decl.Address.IP == "" ||
			decl.Address.Port == "" ||
			len(decl.Endpoints) == 0 ||
			decl.Transport == cluster.EmptyTransport {
			continue
		}

		addr := net.JoinHostPort(decl.Address.IP, decl.Address.Port)

		for _, ep := range decl.Endpoints {
			if !strings.HasPrefix(ep.Path, "/") {
				ep.Path = "/" + ep.Path
			}

			transport := decl.Transport
			if decl.Transport == "" {
				transport = cluster.GrpcTransport
			}

			withApiPrefix := strings.HasPrefix(ep.Path, domain.ApiPrefix)
			cleanPath := strings.TrimPrefix(ep.Path, domain.ApiPrefix)

			method := ep.HttpMethod
			if transport == cluster.GrpcTransport &&
				method == "" {
				method = http.MethodPost
			}

			key := domain.RouteKey{
				Method:        method,
				CleanPath:     cleanPath,
				WithApiPrefix: withApiPrefix,
			}

			route, ok := routes[key]
			if !ok {
				route = &domain.RouteConfig{
					Transport: transport,
				}
				routes[key] = route
			}

			if route.Transport != transport {
				l.logger.Error(
					context.Background(),
					"transport conflict for route",
					log.String("method", key.Method),
					log.String("path", key.CleanPath),
					log.String("transport1", route.Transport),
					log.String("transport2", transport),
				)
				continue
			}

			route.Addresses = append(route.Addresses, addr)
		}
	}
	return routes
}

func (l Locator) registerEndpoint(
	mux *router.Router,
	routeKey domain.RouteKey,
	proxy Proxy,
	middlewares []http2.Middleware,
) {
	handler := middleware.Entrypoint(maxMessageSize, routeKey.WithApiPrefix, proxy, l.logger)
	for i := range slices.Backward(middlewares) {
		handler = middlewares[i](handler)
	}

	paths := []string{
		domain.ApiPrefix + routeKey.CleanPath, // стандартный маршрут c /api
	}
	if !routeKey.WithApiPrefix {
		paths = append(paths, routeKey.CleanPath) // для межмодульного взаимодействия
	}

	for _, path := range paths {
		func() {
			defer func() {
				_ = recover()
			}()

			mux.Handler(routeKey.Method, path,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx, cancel := context.WithTimeout(r.Context(), proxyTimeout)
					defer cancel()

					err := handler(ctx, w, r)
					if err != nil {
						l.logger.Error(ctx, err)
					}
				}),
			)
		}()
	}
}

// nolint:ireturn
func buildProxy(
	transport string,
	addresses []string,
) (Proxy, error) {
	switch transport {
	case cluster.HttpTransport:
		return proxy.NewHttp(addresses), nil

	case cluster.GrpcTransport:
		cli, err := client.New(addresses, client.WithDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageSize)),
			grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(maxMessageSize)),
		))
		if err != nil {
			return nil, errors.WithMessage(err, "init grpc client")
		}
		return proxy.NewGrpc(cli), nil
	default:
		return nil, errors.Errorf("unknown transport '%s'", transport)
	}
}

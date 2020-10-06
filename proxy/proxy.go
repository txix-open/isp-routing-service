package proxy

import (
	"strings"

	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"isp-routing-service/domain"
	"isp-routing-service/proxy/grpc"
	"isp-routing-service/proxy/health_check"
	"isp-routing-service/proxy/http"
	"isp-routing-service/proxy/websocket"
)

var (
	store          = make([]storeItem, 0)
	locationsStore = make([]storeItem, 0)
)

const (
	httpProtocol        = "http"
	websocketProtocol   = "websocket"
	grpcProtocol        = "grpc"
	healthCheckProtocol = "healthсheck"
)

type (
	Proxy interface {
		ProxyRequest(ctx *fasthttp.RequestCtx, path string) domain.ProxyResponse
		Consumer([]structure.AddressConfiguration) bool
		SkipAuth() bool
		SkipExistCheck() bool
		Close()
	}
	storeItem struct {
		pathPrefix string
		paths      []string // if pathPrefix is empty
		proxy      Proxy
	}
)

func InitProxies(configs structure.RoutingConfig) error {
	store = locationsStore
	for _, config := range configs { //nolint
		ip := config.Address.IP
		if config.HandlersInfo == nil && len(config.Endpoints) > 0 {
			defaultInfo := structure.HandlersInfo{
				Endpoints:      config.Endpoints,
				SkipAuth:       true,
				SkipExistCheck: true,
				Port:           config.Address.Port,
			}
			err := addProxyToStore(grpcProtocol, defaultInfo, ip, config.ModuleName)
			if err != nil {
				return err
			}
		}
		for protocol, info := range config.HandlersInfo {
			err := addProxyToStore(protocol, info, ip, config.ModuleName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func addProxyToStore(protocol string, info structure.HandlersInfo, ip string, moduleName string) error {
	p, err := makeProxy(protocol, info.SkipAuth, info.SkipExistCheck)
	if err != nil {
		return errors.Wrapf(err, "bad dynamic config in service %s with protocol %s", moduleName, protocol)
	}
	addressConfig := []structure.AddressConfiguration{{
		Port: info.Port,
		IP:   ip,
	}}
	p.Consumer(addressConfig)
	item := storeItem{
		pathPrefix: "",
		proxy:      p,
		paths:      getPathsFromEndpoints(info.Endpoints),
	}
	store = append(store, item)
	return nil
}

func getPathsFromEndpoints(endpoints []structure.EndpointDescriptor) []string {
	paths := make([]string, len(endpoints)-1)
	for i := range endpoints {
		endpoint := endpoints[i]
		paths = append(paths, endpoint.Path)
	}
	return paths
}

func makeProxy(protocol string, skipAuth, skipExistCheck bool) (Proxy, error) {
	var proxy Proxy
	switch protocol {
	case httpProtocol:
		proxy = http.NewProxy(skipAuth, skipExistCheck)
	case grpcProtocol:
		proxy = grpc.NewProxy(skipAuth, skipExistCheck)
	case healthCheckProtocol:
		proxy = health_check.NewProxy(skipAuth, skipExistCheck)
	case websocketProtocol:
		proxy = websocket.NewProxy(skipAuth, skipExistCheck)
	default:
		return nil, errors.Errorf("unknown protocol '%s'", protocol)
	}

	return proxy, nil
}

func Find(path string) (Proxy, string) {
	for _, item := range store {
		if item.pathPrefix != "" {
			if strings.HasPrefix(path, item.pathPrefix) {
				return item.proxy, getPathWithoutPrefix(path, item.pathPrefix)
			}
		}
		for _, iPath := range item.paths {
			if path == iPath {
				return item.proxy, path
			}
			if path == "/"+iPath {
				return item.proxy, path[1:]
			}
		}
	}
	return nil, ""
}

func Close() {
	for _, p := range store {
		p.proxy.Close()
	}
}

func getPathWithoutPrefix(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

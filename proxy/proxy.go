package proxy

import (
	"strings"

	"github.com/integration-system/isp-lib/v2/structure"
	log "github.com/integration-system/isp-log"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"isp-routing-service/domain"
	"isp-routing-service/proxy/grpc"
	"isp-routing-service/proxy/health_check"
	"isp-routing-service/proxy/http"
	"isp-routing-service/proxy/websocket"
)

var (
	store = make(map[string][]storeItem, 0)
)

const (
	HttpProtocol        = "http"
	WebsocketProtocol   = "websocket"
	GrpcProtocol        = "grpc"
	HealthCheckProtocol = "healthсheck"
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
		proxy    Proxy
		paths    []string
		protocol string
	}
	ModuleInfo struct {
		Paths          []string
		Addresses      []structure.AddressConfiguration
		SkipAuth       bool
		SkipExistCheck bool
	}
	FullModuleInfo map[string]map[string]ModuleInfo
)

func InitProxies(configs FullModuleInfo) error {
	for moduleName, protocolModuleInfo := range configs {
		for protocol, info := range protocolModuleInfo {
			item, err := getProxyStoreItem(moduleName, protocol, info)
			if err != nil {
				//TODO err code
				log.Error(400, err)
			}
			store[moduleName] = append(store[moduleName], item)
		}

	}
	return nil
}

func getProxyStoreItem(moduleName string, protocol string, protocolModuleInfo ModuleInfo) (storeItem, error) {
	p, err := makeProxy(protocol, protocolModuleInfo.SkipAuth, protocolModuleInfo.SkipExistCheck)
	if err != nil {
		return storeItem{}, errors.Wrapf(err, "bad dynamic config in service %s with protocol %s", moduleName, protocol)
	}
	p.Consumer(protocolModuleInfo.Addresses)
	item := storeItem{
		proxy:    p,
		protocol: protocol,
		paths:    protocolModuleInfo.Paths,
	}
	return item, nil
}

func makeProxy(protocol string, skipAuth, skipExistCheck bool) (Proxy, error) {
	var proxy Proxy
	switch protocol {
	case HttpProtocol:
		proxy = http.NewProxy(skipAuth, skipExistCheck)
	case GrpcProtocol:
		proxy = grpc.NewProxy(skipAuth, skipExistCheck)
	case HealthCheckProtocol:
		proxy = health_check.NewProxy(skipAuth, skipExistCheck)
	case WebsocketProtocol:
		proxy = websocket.NewProxy(skipAuth, skipExistCheck)
	default:
		return nil, errors.Errorf("unknown protocol '%s'", protocol)
	}

	return proxy, nil
}

func Find(path string) (Proxy, string) {
	moduleName := strings.Split(path, "/")[0]
	items := store[moduleName]
	for _, item := range items {
		//TODO path prefix
		//if item.pathPrefix != "" {
		//	if strings.HasPrefix(path, item.pathPrefix) {
		//		return item.proxy, getPathWithoutPrefix(path, item.pathPrefix)
		//	}
		//}
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
	for _, items := range store {
		for _, item := range items {
			item.proxy.Close()
		}
	}
}

func getPathWithoutPrefix(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

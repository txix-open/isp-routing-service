package proxy

import (
	"strings"
	"sync"

	"github.com/google/go-cmp/cmp"
	"github.com/integration-system/isp-lib/v2/structure"
	log "github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
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
	mu    sync.RWMutex
)

const (
	HttpProtocol        = "http"
	WebsocketProtocol   = "websocket"
	GrpcProtocol        = "grpc"
	HealthCheckProtocol = "healthсheck"
	noNeedToReplace     = -1
)

type (
	Proxy interface {
		ProxyRequest(ctx *fasthttp.RequestCtx, path string) domain.ProxyResponse
		Consumer([]structure.AddressConfiguration) bool
		Close()
	}
	storeItem struct {
		proxy      Proxy
		paths      []string
		protocol   string
		pathPrefix string // for backward compatibility
		addresses  []structure.AddressConfiguration
	}
	ModuleInfo struct {
		Paths      []string
		Addresses  []structure.AddressConfiguration
		PathPrefix string // for backward compatibility
	}
	FullModuleInfo map[string]map[string]ModuleInfo
)

func InitProxies(configs FullModuleInfo) error {
	mu.Lock()
	defer mu.Unlock()
	for moduleName, protocolModuleInfo := range configs {
		for protocol, info := range protocolModuleInfo {
			needToAdd, indexOfReplacingElem := isProxyNeedReplace(moduleName, protocol, info)
			if needToAdd {
				item, err := getProxyStoreItem(moduleName, protocol, info)
				if err != nil {
					log.Error(stdcodes.ReceiveErrorFromConfig, err)
				}
				store[moduleName] = append(store[moduleName], item)
			}
			if indexOfReplacingElem != noNeedToReplace {
				item, err := getProxyStoreItem(moduleName, protocol, info)
				if err != nil {
					log.Error(stdcodes.ReceiveErrorFromConfig, err)
				}
				store[moduleName][indexOfReplacingElem] = item
			}
		}
	}
	if len(configs) < len(store) { // if service disconnected from config
		for moduleName := range store {
			_, in := configs[moduleName]
			if !in {
				delete(store, moduleName)
				continue
			}
		}
	}
	return nil
}

func isProxyNeedReplace(moduleName string, protocol string, info ModuleInfo) (needToAdd bool, indexOfReplacingElem int) {
	storeItem := store[moduleName]
	for i, el := range storeItem {
		if el.protocol == protocol {
			if el.pathPrefix == info.PathPrefix && cmp.Equal(el.addresses, info.Addresses) {
				return false, noNeedToReplace
			}
			if !cmp.Equal(el.paths, info.Paths) || el.pathPrefix != info.PathPrefix || !cmp.Equal(el.addresses, info.Addresses) {
				return false, i
			}
		}
	}
	return true, noNeedToReplace
}

func getProxyStoreItem(moduleName string, protocol string, protocolModuleInfo ModuleInfo) (storeItem, error) {
	p, err := makeProxy(protocol)
	if err != nil {
		return storeItem{}, errors.Wrapf(err, "bad dynamic config in service %s with protocol %s", moduleName, protocol)
	}
	p.Consumer(protocolModuleInfo.Addresses)
	item := storeItem{
		proxy:      p,
		protocol:   protocol,
		paths:      protocolModuleInfo.Paths,
		pathPrefix: protocolModuleInfo.PathPrefix,
		addresses:  protocolModuleInfo.Addresses,
	}
	return item, nil
}

func makeProxy(protocol string) (Proxy, error) {
	var proxy Proxy
	switch protocol {
	case HttpProtocol:
		proxy = http.NewProxy()
	case GrpcProtocol:
		proxy = grpc.NewProxy()
	case HealthCheckProtocol:
		proxy = health_check.NewProxy()
	case WebsocketProtocol:
		proxy = websocket.NewProxy()
	default:
		return nil, errors.Errorf("unknown protocol '%s'", protocol)
	}

	return proxy, nil
}

func Find(path string) (Proxy, string) {
	mu.Lock()
	defer mu.Unlock()
	moduleNameSplited := strings.Split(path, "/")
	items := store[moduleNameSplited[1]]
	if items == nil && len(moduleNameSplited) > 2 {
		items = store[moduleNameSplited[2]]
	}
	for _, item := range items {
		if item.pathPrefix != "" {
			if path[0] == '/' {
				path = path[1:]
			}
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

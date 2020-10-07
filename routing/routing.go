package routing

import (
	"github.com/integration-system/isp-lib/v2/structure"
	"isp-routing-service/proxy"
)

func ParseConfig(configs structure.RoutingConfig) proxy.FullModuleInfo {
	fullInfo := make(proxy.FullModuleInfo, 0)
	for _, config := range configs {
		ip := config.Address.IP
		moduleName := config.ModuleName
		if config.HandlersInfo == nil {
			config.HandlersInfo = map[string]structure.HandlersInfo{
				proxy.GrpcProtocol: {
					Endpoints:      config.Endpoints,
					SkipAuth:       false,
					SkipExistCheck: false,
					Port:           config.Address.Port,
				},
			}
		}

		for protocol, info := range config.HandlersInfo {
			if len(info.Endpoints) < 1 {
				continue
			}
			if fullInfo[moduleName] == nil {
				fullInfo[moduleName] = map[string]proxy.ModuleInfo{
					protocol: {
						Paths:          getPathsFromEndpoints(info.Endpoints),
						Addresses:      []structure.AddressConfiguration{{info.Port, ip}},
						SkipAuth:       info.SkipAuth,
						SkipExistCheck: info.SkipExistCheck,
					},
				}
			} else {
				el, in := fullInfo[moduleName][protocol]
				if !in {
					fullInfo[moduleName][protocol] = proxy.ModuleInfo{
						Paths:          getPathsFromEndpoints(info.Endpoints),
						Addresses:      []structure.AddressConfiguration{{info.Port, ip}},
						SkipAuth:       info.SkipAuth,
						SkipExistCheck: info.SkipExistCheck,
						PathPrefix:     info.PathPrefix,
					}
				} else {
					el.Addresses = append(el.Addresses, structure.AddressConfiguration{
						Port: info.Port,
						IP:   ip,
					})
					fullInfo[moduleName][protocol] = el
				}

			}
		}
	}
	return fullInfo
}

func getPathsFromEndpoints(endpoints []structure.EndpointDescriptor) []string {
	paths := make([]string, len(endpoints))
	for i := range endpoints {
		endpoint := endpoints[i]
		paths[i] = endpoint.Path
	}
	return paths
}

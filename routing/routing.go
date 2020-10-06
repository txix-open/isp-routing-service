package routing

import (
	"github.com/integration-system/isp-lib/v2/structure"
)

var (
	InnerMethods    = make(map[string]bool)
	AllMethods      = make(map[string]bool)
	AuthUserMethods = make(map[string]bool)
)

func InitRoutes(configs structure.RoutingConfig) {
	newAddressMap := make(map[string]bool)
	newInnerAddressMap := make(map[string]bool)
	newAuthUserAddressMap := make(map[string]bool)
	for i := range configs {
		backend := &configs[i]
		if backend.Address.IP == "" {
			continue
		}
		if backend.HandlersInfo == nil {
			if len(backend.Endpoints) == 0 || backend.Address.Port == "" {
				continue
			}
			addEndpointsToMaps(backend.Endpoints, newAddressMap, newInnerAddressMap, newAuthUserAddressMap)
		} else {
			for _, info := range backend.HandlersInfo {
				if info.Port != "" {
					addEndpointsToMaps(info.Endpoints, newAddressMap, newInnerAddressMap, newAuthUserAddressMap)
				}
			}
		}
	}
	AllMethods = newAddressMap
	InnerMethods = newInnerAddressMap
	AuthUserMethods = newAuthUserAddressMap
}

func addEndpointsToMaps(endpoints []structure.EndpointDescriptor, newAddressMap map[string]bool, newInnerAddressMap map[string]bool, newAuthUserAddressMap map[string]bool) { //nolint
	for _, el := range endpoints {
		newAddressMap[el.Path] = true
		if el.Inner {
			newInnerAddressMap[el.Path] = true
		}
		if el.UserAuthRequired {
			newAuthUserAddressMap[el.Path] = true
		}
	}
}

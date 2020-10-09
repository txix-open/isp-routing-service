package conf

import "github.com/integration-system/isp-lib/v2/structure"

type Configuration struct {
	InstanceUuid         string                         `valid:"required~Required" json:"configServiceAddress"`
	ModuleName           string                         `valid:"required~Required" json:"moduleName"`
	ConfigServiceAddress structure.AddressConfiguration `valid:"required~Required" json:"instanceUuid"`
	HttpOuterAddress     structure.AddressConfiguration `valid:"required~Required" json:"httpOuterAddress"`
	HttpInnerAddress     structure.AddressConfiguration `valid:"required~Required" json:"httpInnerAddress"`
}

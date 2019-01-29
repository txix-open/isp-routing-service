package conf

import "github.com/integration-system/isp-lib/structure"

type Configuration struct {
	InstanceUuid         string                         `valid:"required~Required" json:"configServiceAddress"`
	ModuleName           string                         `valid:"required~Required" json:"moduleName"`
	ConfigServiceAddress structure.AddressConfiguration `valid:"required~Required" json:"instanceUuid"`
	GrpcOuterAddress     structure.AddressConfiguration `valid:"required~Required" json:"grpcOuterAddress"`
	GrpcInnerAddress     structure.AddressConfiguration `valid:"required~Required" json:"grpcInnerAddress"`
}

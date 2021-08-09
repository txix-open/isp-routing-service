package conf

import (
	"github.com/integration-system/isp-lib/v2/config"
	"github.com/integration-system/isp-lib/v2/structure"
)

type Configuration struct {
	config.CommonLocalConfig
	GrpcOuterAddress structure.AddressConfiguration `valid:"required~Required" json:"grpcOuterAddress"`
	GrpcInnerAddress structure.AddressConfiguration `valid:"required~Required" json:"grpcInnerAddress"`
}

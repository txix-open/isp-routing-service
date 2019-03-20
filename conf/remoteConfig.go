package conf

import (
	"github.com/integration-system/isp-lib/structure"
)

type RemoteConfig struct {
	Database structure.DBConfiguration     `schema:"Admin service database" valid:"required~Required" json:"database"`
	Metrics  structure.MetricConfiguration `schema:"Metrics"`
}

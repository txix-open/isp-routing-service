package conf

import (
	"github.com/integration-system/isp-lib/v2/structure"
)

type RemoteConfig struct {
	Metrics structure.MetricConfiguration `schema:"Настройка метрик"`
}

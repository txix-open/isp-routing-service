package conf

import (
	"github.com/integration-system/isp-lib/structure"
)

type RemoteConfig struct {
	Metrics structure.MetricConfiguration `schema:"Настройка метрик"`
}

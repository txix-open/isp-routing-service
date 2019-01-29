package routing

import (
	"github.com/integration-system/isp-lib/metric"
	"github.com/rcrowley/go-metrics"
	"time"
)

var (
	mh *metricHolder
)

type metricHolder struct {
	total metrics.Histogram
}

type timer struct {
	updater func(int64)
	start   int64
}

func (t *timer) End() {
	t.updater((time.Now().UnixNano() - t.start) / 1000000)
}

func (mh *metricHolder) StartTotalTimer() *timer {
	return &timer{
		updater: mh.total.Update,
		start:   time.Now().UnixNano(),
	}
}

func startTimer(method string) *timer {
	histogram := metrics.GetOrRegisterHistogram(
		"grpc.response.time_"+method, metric.GetRegistry(), nil,
	)
	return &timer{
		updater: histogram.Update,
		start:   time.Now().UnixNano(),
	}
}

func incCounter(code string) {
	metrics.GetOrRegisterCounter("grpc.response.count."+code, metric.GetRegistry()).Inc(1)
}

func ensureHistogramForMethod(method string) {
	metrics.GetOrRegisterHistogram(
		"grpc.response.time_"+method, metric.GetRegistry(), metrics.NewUniformSample(1028),
	)
}

func InitMetrics() {
	if mh == nil {
		mh = &metricHolder{
			total: metrics.GetOrRegisterHistogram(
				"grpc.response.time", metric.GetRegistry(), metrics.NewUniformSample(1028),
			),
		}
	}
}

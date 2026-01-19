package tests_test

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"isp-routing-service/assembly"
	"isp-routing-service/service/proxy"

	"github.com/stretchr/testify/require"
	"github.com/txix-open/isp-kit/cluster"
	"github.com/txix-open/isp-kit/log"
	"github.com/txix-open/isp-kit/test"
	"github.com/txix-open/isp-kit/test/grpct"
	"github.com/txix-open/isp-kit/test/httpt"
)

func BenchmarkProxy_HTTP(b *testing.B) {
	test, _ := test.New(&testing.T{})
	require := require.New(b)

	logger, err := log.New(log.WithDisableDefaultOutput())
	require.NoError(err)

	mockServ := httpt.NewMock(test)
	mockServ.Wrapper.Middlewares = nil
	mockServ.Mock(http.MethodGet, "/alive_backend/endpoint", func() string { return "OK" })

	hostPort := strings.TrimPrefix(mockServ.BaseURL(), "http://")
	h, p, err := net.SplitHostPort(hostPort)
	require.NoError(err)

	routing := cluster.RoutingConfig{
		cluster.BackendDeclaration{
			ModuleName: "alive_backend",
			Version:    "1.0.0",
			LibVersion: "1.0.0",
			Transport:  "http",
			Endpoints: []cluster.EndpointDescriptor{
				{HttpMethod: http.MethodGet, Path: "alive_backend/endpoint"},
			},
			Address: cluster.AddressConfiguration{
				IP:   h,
				Port: p,
			},
		},
	}

	locator := assembly.NewLocator(logger)
	cfg := locator.LocatorConfig(routing, make(map[assembly.ProxyKey]assembly.Proxy))

	_, proxyCli := httpt.TestServer(test, cfg.Handler)

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(32)
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			_ = proxyCli.Get("/api/alive_backend/endpoint").
				DoWithoutResponse(b.Context())
		}
	})
}

func BenchmarkProxy_GRPC(b *testing.B) {
	test, _ := test.New(&testing.T{})
	require := require.New(b)

	logger, err := log.New(log.WithDisableDefaultOutput())
	require.NoError(err)

	routing := cluster.RoutingConfig{
		cluster.BackendDeclaration{
			ModuleName: "alive_backend",
			Version:    "1.0.0",
			LibVersion: "1.0.0",
			Endpoints: []cluster.EndpointDescriptor{
				{Path: "alive_backend/endpoint"},
			},
			Address: cluster.AddressConfiguration{
				IP:   "host",
				Port: "port",
			},
		},
	}

	mock, mockCli := grpct.NewMock(test)
	mock.Mock("alive_backend/endpoint", func() string { return "OK" })

	locator := assembly.NewLocator(logger)
	reuseProxy := map[assembly.ProxyKey]assembly.Proxy{
		{Transport: "grpc", Addresses: "host:port"}: proxy.NewGrpc(mockCli),
	}
	cfg := locator.LocatorConfig(routing, reuseProxy)

	_, proxyCli := httpt.TestServer(test, cfg.Handler)

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(32)
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			_ = proxyCli.Post("/api/alive_backend/endpoint").
				DoWithoutResponse(b.Context())
		}
	})
}

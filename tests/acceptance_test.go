// nolint:unparam,funlen
package tests_test

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"isp-routing-service/assembly"
	"isp-routing-service/service/proxy"

	"github.com/txix-open/isp-kit/cluster"
	"github.com/txix-open/isp-kit/grpc"
	endpoint2 "github.com/txix-open/isp-kit/grpc/endpoint"
	"github.com/txix-open/isp-kit/grpc/isp"
	"github.com/txix-open/isp-kit/http/httpcli"
	"github.com/txix-open/isp-kit/json"
	"github.com/txix-open/isp-kit/log"
	"github.com/txix-open/isp-kit/test"
	"github.com/txix-open/isp-kit/test/grpct"
	"github.com/txix-open/isp-kit/test/httpt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AcceptanceTestSuite struct {
	suite.Suite

	test      *test.Test
	grpcMock  *GrpcMockServer
	httpMocks []*httpt.MockServer
	proxyCli  *httpcli.Client
	routing   cluster.RoutingConfig
}

func TestAcceptance(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(AcceptanceTestSuite))
}

func (s *AcceptanceTestSuite) SetupSuite() {
	t, require := test.New(s.T())
	s.test = t

	var grpcMockAddr string
	s.grpcMock, grpcMockAddr = newGrpcMockWithEndpoints(t, map[string]any{
		"grpc/alive_backend/endpoint":        func() string { return "GRPC_OK" },
		"api/grpc/alive_backend/endpoint_v2": func() string { return "GRPC_API" },
		"grpc/alive_backend/endpoint_with_error": func() error {
			return status.Error(codes.FailedPrecondition, "test error")
		},
	})
	host, port, err := net.SplitHostPort(grpcMockAddr)
	s.Require().NoError(err)

	s.routing = cluster.RoutingConfig{
		{
			ModuleName: "dead_backend",
			Version:    "1.0.0",
			LibVersion: "1.0.0",
			Transport:  "http",
			Endpoints: []cluster.EndpointDescriptor{
				{HttpMethod: http.MethodPost, Path: "dead_backend/endpoint"},
			},
			Address: cluster.AddressConfiguration{IP: "unknownhost", Port: "0"},
		},
		cluster.BackendDeclaration{
			ModuleName: "alive_backend",
			Version:    "1.0.0",
			LibVersion: "1.0.0",
			Endpoints: []cluster.EndpointDescriptor{
				{Path: "grpc/alive_backend/endpoint"},
				{Path: "api/grpc/alive_backend/endpoint_v2"},
				{Path: "grpc/alive_backend/endpoint_with_error"},
			},
			Address: cluster.AddressConfiguration{
				IP:   host,
				Port: port,
			},
		},
	}

	s.httpMocks = []*httpt.MockServer{}
	handlers := map[string]any{
		"/alive_backend/endpoint": func() string {
			return "HTTP_OK"
		},
		"/api/alive_backend/endpoint_v2": func() string {
			return "HTTP_API"
		},
		"/alive_backend/endpoint_with_error": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "test error", http.StatusPreconditionFailed)
		},
	}
	for range 2 {
		mock := httpt.NewMock(t)
		for path, h := range handlers {
			mock.Mock(http.MethodGet, path, h)
			mock.Mock(http.MethodPost, path, h)
		}
		s.httpMocks = append(s.httpMocks, mock)
	}

	for _, mock := range s.httpMocks {
		hostPort := strings.TrimPrefix(mock.BaseURL(), "http://")
		h, p, err := net.SplitHostPort(hostPort)
		require.NoError(err)
		s.routing = append(s.routing, cluster.BackendDeclaration{
			ModuleName: "alive_backend",
			Version:    "1.0.0",
			LibVersion: "1.0.0",
			Transport:  "http",
			Endpoints: []cluster.EndpointDescriptor{
				{HttpMethod: http.MethodGet, Path: "alive_backend/endpoint"},
				{HttpMethod: http.MethodGet, Path: "/api/alive_backend/endpoint_v2"},
				{HttpMethod: http.MethodPost, Path: "alive_backend/endpoint_with_error"},
			},
			Address: cluster.AddressConfiguration{
				IP:   h,
				Port: p,
			},
		})
	}

	locator := assembly.NewLocator(s.test.Logger())

	mock, mockCli := grpct.NewMock(s.test)
	mock.Mock("reuse_backend/endpoint", func() string { return "REUSE" })

	reuseProxy := map[assembly.ProxyKey]assembly.Proxy{
		{Transport: "grpc", Addresses: "host:port"}: proxy.NewGrpc(mockCli),
	}

	s.routing = append(s.routing,
		cluster.BackendDeclaration{
			ModuleName: "reuse_backend",
			Version:    "1.0.0",
			LibVersion: "1.0.0",
			Endpoints: []cluster.EndpointDescriptor{
				{Path: "reuse_backend/endpoint"},
			},
			Address: cluster.AddressConfiguration{
				IP:   "host",
				Port: "port",
			},
		},
	)

	locatorCfg := locator.LocatorConfig(s.routing, reuseProxy)
	_, s.proxyCli = httpt.TestServer(t, locatorCfg.Handler)
}

func (s *AcceptanceTestSuite) TestUnknownEndpoint() {
	s.assertStatus(http.MethodPost, "/api/unknown_endpoint", http.StatusNotImplemented)
	s.assertStatus(http.MethodPost, "/unknown_endpoint", http.StatusNotImplemented)
}

func (s *AcceptanceTestSuite) TestDeadBackend() {
	s.assertStatus(http.MethodPost, "/api/dead_backend/endpoint", http.StatusServiceUnavailable)
	s.assertStatus(http.MethodPost, "/dead_backend/endpoint", http.StatusServiceUnavailable)
}

func (s *AcceptanceTestSuite) TestGrpcReuseEndpoint() {
	s.assertJsonResponse(http.MethodPost, "api/reuse_backend/endpoint", "REUSE")
	s.assertJsonResponse(http.MethodPost, "reuse_backend/endpoint", "REUSE")
}

func (s *AcceptanceTestSuite) TestGrpcAliveEndpoint() {
	s.assertJsonResponse(http.MethodPost, "api/grpc/alive_backend/endpoint", "GRPC_OK")
	s.assertJsonResponse(http.MethodPost, "grpc/alive_backend/endpoint", "GRPC_OK")
}

func (s *AcceptanceTestSuite) TestGrpcAliveApiEndpoint() {
	s.assertJsonResponse(http.MethodPost, "api/grpc/alive_backend/endpoint_v2", "GRPC_API")
}

func (s *AcceptanceTestSuite) TestGrpcEndpointWithError() {
	s.assertStatus(http.MethodPost, "api/grpc/alive_backend/endpoint_with_error", http.StatusPreconditionFailed)
	s.assertStatus(http.MethodPost, "grpc/alive_backend/endpoint_with_error", http.StatusPreconditionFailed)
}

func (s *AcceptanceTestSuite) TestHttpAliveEndpoint() {
	s.assertJsonResponse(http.MethodGet, "/api/alive_backend/endpoint", "HTTP_OK")
	s.assertJsonResponse(http.MethodGet, "/alive_backend/endpoint", "HTTP_OK")
}

func (s *AcceptanceTestSuite) TestHttpAliveApiEndpoint() {
	s.assertJsonResponse(http.MethodGet, "/api/alive_backend/endpoint_v2", "HTTP_API")
}

func (s *AcceptanceTestSuite) TestHttpEndpointWithError() {
	s.assertStatus(http.MethodPost, "/api/alive_backend/endpoint_with_error", http.StatusPreconditionFailed)
	s.assertStatus(http.MethodPost, "/alive_backend/endpoint_with_error", http.StatusPreconditionFailed)
}

func (s *AcceptanceTestSuite) assertStatus(method, path string, expected int) {
	reqBuilder := httpcli.NewRequestBuilder(method, path, s.proxyCli.GlobalRequestConfig(), s.proxyCli.Execute)
	resp, err := reqBuilder.Do(s.T().Context())
	s.Require().NoError(err)
	s.Require().Equal(expected, resp.StatusCode())
	resp.Close()
}

func (s *AcceptanceTestSuite) assertJsonResponse(method, path string, expected any) {
	var resultRaw json.RawMessage

	reqBuilder := httpcli.NewRequestBuilder(method, path, s.proxyCli.GlobalRequestConfig(), s.proxyCli.Execute)
	err := reqBuilder.StatusCodeToError().JsonResponseBody(&resultRaw).DoWithoutResponse(s.T().Context())
	s.Require().NoError(err)

	expectedRaw, err := json.Marshal(expected)
	s.Require().NoError(err)

	s.Require().JSONEq(string(expectedRaw), string(resultRaw))
}

type GrpcMockServer struct {
	srv           *grpc.Server
	logger        log.Logger
	mockEndpoints map[string]any
}

func newGrpcMockWithEndpoints(t *test.Test, endpoints map[string]any) (*GrpcMockServer, string) {
	srv, addr := grpcServer(t, grpc.NewMux())
	mock := &GrpcMockServer{
		srv:           srv,
		logger:        t.Logger(),
		mockEndpoints: make(map[string]any),
	}
	for ep, h := range endpoints {
		mock.Mock(ep, h)
	}
	return mock, addr
}

func (m *GrpcMockServer) Mock(endpoint string, handler any) *GrpcMockServer {
	m.mockEndpoints[endpoint] = handler
	wrapper := endpoint2.DefaultWrapper(m.logger)
	muxer := grpc.NewMux()
	for e, h := range m.mockEndpoints {
		muxer.Handle(e, wrapper.Endpoint(h))
	}
	m.srv.Upgrade(muxer)
	return m
}

func grpcServer(t *test.Test, service isp.BackendServiceServer) (*grpc.Server, string) {
	assert := t.Assert()

	// nolint:noctx
	listener, err := net.Listen("tcp", "127.0.0.1:")
	assert.NoError(err)
	srv := grpc.NewServer()
	assert.NoError(err)
	t.T().Cleanup(func() {
		srv.Shutdown()
	})
	srv.Upgrade(service)
	go func() {
		err := srv.Serve(listener)
		assert.NoError(err)
	}()
	return srv, listener.Addr().String()
}

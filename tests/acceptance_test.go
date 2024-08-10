package tests

import (
	"context"
	"net"
	"testing"

	"github.com/integration-system/isp-kit/cluster"
	"github.com/integration-system/isp-kit/grpc"
	"github.com/integration-system/isp-kit/grpc/client"
	endpoint2 "github.com/integration-system/isp-kit/grpc/endpoint"
	"github.com/integration-system/isp-kit/grpc/isp"
	"github.com/integration-system/isp-kit/log"
	"github.com/integration-system/isp-kit/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"isp-routing-service/assembly"
	"isp-routing-service/service"
)

func TestAcceptance(t *testing.T) {
	test, require := test.New(t)

	targetSrv, targetAddr := NewMock(test)
	targetSrv.Mock("alive_backend/endpoint", func() string {
		return "OK"
	}).Mock("alive_backend/endpoint_with_error", func() error {
		return status.Error(codes.FailedPrecondition, "test error")
	})

	director := service.NewDirector(test.Logger())
	targetHost, targetPort, err := net.SplitHostPort(targetAddr)
	require.NoError(err)
	routing := cluster.RoutingConfig{{
		ModuleName: "alive_backend",
		Version:    "1.0.0",
		LibVersion: "1.0.0",
		Endpoints: []cluster.EndpointDescriptor{{
			Path: "alive_backend/endpoint",
		}, {
			Path: "alive_backend/endpoint_with_error",
		}},
		Address: cluster.AddressConfiguration{
			IP:   targetHost,
			Port: targetPort,
		},
	}, {
		ModuleName: "dead_backend",
		Version:    "1.0.0",
		LibVersion: "1.0.0",
		Endpoints: []cluster.EndpointDescriptor{{
			Path: "dead_backend/endpoint",
		}},
		Address: cluster.AddressConfiguration{
			IP:   "unknownhost",
			Port: targetPort,
		},
	}}
	director.Upgrade(routing)
	proxyServer := assembly.NewGrpcProxyServer(director)
	proxyListener, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(err)
	go func() {
		err := proxyServer.Serve(proxyListener)
		require.NoError(err)
	}()

	proxyCli, err := client.Default()
	require.NoError(err)
	proxyCli.Upgrade([]string{proxyListener.Addr().String()})

	err = proxyCli.Invoke("unknown_endpoint").Do(context.Background())
	require.Error(err)
	require.EqualValues(codes.Unimplemented, status.Code(err))

	err = proxyCli.Invoke("dead_backend/endpoint").Do(context.Background())
	require.Error(err)
	require.EqualValues(codes.Unavailable, status.Code(err))

	resp := ""
	err = proxyCli.Invoke("alive_backend/endpoint").JsonResponseBody(&resp).Do(context.Background())
	require.NoError(err)
	require.EqualValues("OK", resp)

	err = proxyCli.Invoke("alive_backend/endpoint_with_error").Do(context.Background())
	require.Error(err)
	require.EqualValues(codes.FailedPrecondition, status.Code(err))
}

type MockServer struct {
	srv           *grpc.Server
	logger        log.Logger
	mockEndpoints map[string]interface{}
}

func NewMock(t *test.Test) (*MockServer, string) {
	srv, addr := server(t, grpc.NewMux())
	return &MockServer{
		srv:           srv,
		logger:        t.Logger(),
		mockEndpoints: make(map[string]interface{}),
	}, addr
}

func (m *MockServer) Mock(endpoint string, handler interface{}) *MockServer {
	m.mockEndpoints[endpoint] = handler
	wrapper := endpoint2.DefaultWrapper(m.logger)
	muxer := grpc.NewMux()
	for e, handler := range m.mockEndpoints {
		muxer.Handle(e, wrapper.Endpoint(handler))
	}
	m.srv.Upgrade(muxer)
	return m
}

func server(t *test.Test, service isp.BackendServiceServer) (*grpc.Server, string) {
	assert := t.Assert()

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

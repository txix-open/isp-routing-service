package main

import (
	"github.com/integration-system/golang-socketio"
	"github.com/integration-system/isp-lib/config/schema"
	"github.com/integration-system/isp-lib/structure"
	"net"
	"os"
	"time"

	"isp-routing-service/conf"
	"isp-routing-service/routing"

	"isp-routing-service/controller"
	"isp-routing-service/generated"

	"context"
	"github.com/integration-system/isp-lib/bootstrap"
	"github.com/integration-system/isp-lib/grpc-proxy"
	"github.com/integration-system/isp-lib/logger"
	"github.com/integration-system/isp-lib/metric"
	"google.golang.org/grpc"
	"sync"
)

var (
	server  *grpc.Server
	lock    = sync.RWMutex{}
	version = "0.1.0"
	date    = "undefined"
)

func main() {
	bootstrap.
		ServiceBootstrap(&conf.Configuration{}, &conf.RemoteConfig{}).
		OnLocalConfigLoad(func(cfg *conf.Configuration) {
			startGrpcServer(cfg)
		}).
		DefaultRemoteConfigPath(schema.ResolveDefaultConfigPath("default_remote_config.json")).
		RequireRoutes(handleRouteUpdate).
		OnSocketEvent(gosocketio.OnDisconnection, func(_ *gosocketio.Channel) {
			routing.MarkUninitialized()
		}).
		DeclareMe(routesData).
		SocketConfiguration(socketConfiguration).
		OnRemoteConfigReceive(onRemoteConfigReceive).
		OnShutdown(onShutdown).
		Run()
}

func stopGrpcServer() {
	lock.Lock()
	if server != nil {
		server.GracefulStop()
		server = nil
	}
	lock.Unlock()
}

func startGrpcServer(cfg *conf.Configuration) {
	grpcAddress := cfg.GrpcInnerAddress.GetAddress()
	var lis net.Listener
	var err error
	counter := 0
	go func() {
		for lis, err = net.Listen("tcp", grpcAddress); err != nil; {
			counter++
			time.Sleep(time.Second * time.Duration(counter))
			logger.Infof("Error grpc connection: %v, try again, err: %v", grpcAddress, err)
		}
		h := grpc_proxy.TransparentHandler(routing.GetRouter())
		server = grpc.NewServer(
			grpc.CustomCodec(grpc_proxy.Codec()),
			grpc.UnknownServiceHandler(h))
		grpcServer := controller.GetGRPCServer()
		generated.RegisterRoutingServiceServer(server, &grpcServer)
		logger.Infof("Start grpc server on %s", grpcAddress)
		if err := server.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
		logger.Info("Grpc server shutdown")
	}()
}

func handleRouteUpdate(configs structure.RoutingConfig) bool {
	firstInit, hasErrors := routing.InitRoutes(configs)
	if firstInit && hasErrors {
		logger.Fatal("Received unreachable route while initializing. Shutdown now.")
	}
	return true
}

func socketConfiguration(cfg interface{}) structure.SocketConfiguration {
	appConfig := cfg.(*conf.Configuration)
	return structure.SocketConfiguration{
		Host:   appConfig.ConfigServiceAddress.IP,
		Port:   appConfig.ConfigServiceAddress.Port,
		Secure: false,
		UrlParams: map[string]string{
			"module_name":   appConfig.ModuleName,
			"instance_uuid": appConfig.InstanceUuid,
		},
	}
}

func onShutdown(_ context.Context, _ os.Signal) {
	stopGrpcServer()
}

func onRemoteConfigReceive(remoteConfig, oldRemoteConfig *conf.RemoteConfig) {
	metric.InitCollectors(remoteConfig.Metrics, oldRemoteConfig.Metrics)
	metric.InitHttpServer(remoteConfig.Metrics)
	routing.InitMetrics()
}

func routesData(localConfig interface{}) bootstrap.ModuleInfo {
	cfg := localConfig.(*conf.Configuration)
	return bootstrap.ModuleInfo{
		ModuleName:       cfg.ModuleName,
		ModuleVersion:    version,
		GrpcOuterAddress: cfg.GrpcOuterAddress,
		Handlers:         []interface{}{},
	}
}

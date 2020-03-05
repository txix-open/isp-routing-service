package main

import (
	"context"
	"github.com/vgough/grpc-proxy/proxy"
	"net"
	"os"
	"sync"
	"time"

	"isp-routing-service/conf"
	"isp-routing-service/log_code"
	"isp-routing-service/routing"

	"github.com/integration-system/isp-lib/v2/bootstrap"
	"github.com/integration-system/isp-lib/v2/config/schema"

	"github.com/integration-system/isp-lib/v2/metric"
	"github.com/integration-system/isp-lib/v2/structure"
	log "github.com/integration-system/isp-log"
	"google.golang.org/grpc"
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
		for lis, err = net.Listen("tcp", grpcAddress); err != nil; lis, err = net.Listen("tcp", grpcAddress) {
			log.WithMetadata(map[string]interface{}{
				log_code.MdAddr: grpcAddress,
			}).Infof(log_code.FatalGrpcServerFailedConnection, "error grpc connection; try again, err: %v", err)
			counter++
			time.Sleep(time.Second * time.Duration(counter))
		}
		h := proxy.TransparentHandler(routing.GetRouter())
		server = grpc.NewServer(
			grpc.CustomCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(h))
		log.WithMetadata(map[string]interface{}{
			log_code.MdAddr: grpcAddress,
		}).Info(log_code.InfoGrpcServerStart, "start grpc server")
		if err := server.Serve(lis); err != nil {
			log.Fatalf(log_code.FatalGrpcServerFailedConnection, "failed to serve: %v", err)
		}
		log.Info(log_code.InfoGrpcServerShutdown, "grpc server shutdown")
	}()
}

func handleRouteUpdate(configs structure.RoutingConfig) bool {
	firstInit, hasErrors := routing.InitRoutes(configs)
	if firstInit && hasErrors {
		log.Fatal(log_code.FatalHandleRouteUpdate, "received unreachable route while initializing. shutdown now.")
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

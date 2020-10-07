package main

import (
	"context"
	"os"

	"github.com/integration-system/isp-lib/v2/bootstrap"
	"github.com/integration-system/isp-lib/v2/config/schema"
	"github.com/integration-system/isp-lib/v2/structure"
	log "github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
	"isp-routing-service/conf"
	"isp-routing-service/proxy"
	"isp-routing-service/routing"
	"isp-routing-service/server"
)

var (
	version = "0.1.0"
)

func main() {
	bootstrap.
		ServiceBootstrap(&conf.Configuration{}, &conf.RemoteConfig{}).
		OnLocalConfigLoad(onLocalConfigLoad).
		DefaultRemoteConfigPath(schema.ResolveDefaultConfigPath("default_remote_config.json")).
		RequireRoutes(handleRouteUpdate).
		DeclareMe(makeDeclaration).
		SocketConfiguration(socketConfiguration).
		OnRemoteConfigReceive(onRemoteConfigReceive).
		OnShutdown(onShutdown).
		Run()
}

func onLocalConfigLoad(_ *conf.Configuration) {

}

func onRemoteConfigReceive(remoteConfig, oldRemoteConfig *conf.RemoteConfig) {
	server.Http.Init(remoteConfig.HttpSetting, oldRemoteConfig.HttpSetting)
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
	server.Http.Close()
	proxy.Close()
}

func handleRouteUpdate(configs structure.RoutingConfig) bool {
	modulesInfo := routing.ParseConfig(configs)
	err := proxy.InitProxies(modulesInfo)
	if err != nil {
		log.Error(stdcodes.ReceiveErrorFromConfig, err)
	}
	return true
}

func makeDeclaration(localConfig interface{}) bootstrap.ModuleInfo {
	cfg := localConfig.(*conf.Configuration)
	return bootstrap.ModuleInfo{
		ModuleName:       cfg.ModuleName,
		ModuleVersion:    version,
		GrpcOuterAddress: cfg.HttpOuterAddress,
		Endpoints:        []structure.EndpointDescriptor{},
	}
}

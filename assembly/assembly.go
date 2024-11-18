package assembly

import (
	"context"
	"net"

	"github.com/pkg/errors"
	"github.com/txix-open/isp-kit/app"
	"github.com/txix-open/isp-kit/bootstrap"
	"github.com/txix-open/isp-kit/cluster"
	"github.com/vgough/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"isp-routing-service/service"

	"github.com/txix-open/isp-kit/log"
)

type Assembly struct {
	director *service.Director
	boot     *bootstrap.Bootstrap
	server   *grpc.Server
	logger   *log.Adapter
}

func New(boot *bootstrap.Bootstrap) *Assembly {
	director := service.NewDirector()
	return &Assembly{
		director: director,
		boot:     boot,
		server:   NewGrpcProxyServer(director),
		logger:   boot.App.Logger(),
	}
}

func (a *Assembly) ReceiveConfig(_ context.Context, _ []byte) error {
	return nil
}

func (a *Assembly) ReceiveRoutes(_ context.Context, routes cluster.RoutingConfig) error {
	a.director.Upgrade(a.logger, routes)
	return nil
}

func (a *Assembly) Runners() []app.Runner {
	eventHandler := cluster.NewEventHandler().
		RemoteConfigReceiver(a).
		RoutesReceiver(a)
	return []app.Runner{
		app.RunnerFunc(func(ctx context.Context) error {
			lis, err := net.Listen("tcp", a.boot.BindingAddress)
			if err != nil {
				return errors.WithMessagef(err, "listen %s", a.boot.BindingAddress)
			}
			return a.server.Serve(lis)
		}),
		app.RunnerFunc(func(ctx context.Context) error {
			return a.boot.ClusterCli.Run(ctx, eventHandler)
		}),
	}
}

func (a *Assembly) Closers() []app.Closer {
	return []app.Closer{
		a.boot.ClusterCli,
		app.CloserFunc(func() error {
			a.server.GracefulStop()
			return nil
		}),
	}
}

func NewGrpcProxyServer(director proxy.StreamDirector) *grpc.Server {
	return grpc.NewServer(
		grpc.ForceServerCodec(proxy.Codec()), //nolint:staticcheck
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director)),
		grpc.MaxRecvMsgSize(service.MaxMessageSize),
		grpc.MaxSendMsgSize(service.MaxMessageSize),
	)
}

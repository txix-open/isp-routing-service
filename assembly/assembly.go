package assembly

import (
	"context"
	mux2 "github.com/gorilla/mux"
	"github.com/integration-system/isp-kit/app"
	"github.com/integration-system/isp-kit/bootstrap"
	"github.com/integration-system/isp-kit/cluster"
	"github.com/integration-system/isp-kit/http"
	"github.com/pkg/errors"
	"github.com/vgough/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"isp-routing-service/conf"
	"isp-routing-service/service"
	"net"

	"github.com/integration-system/isp-kit/log"
)

type Assembly struct {
	director   *service.Director
	boot       *bootstrap.Bootstrap
	grpcServer *grpc.Server
	httpServer *http.Server
	logger     *log.Adapter
	conf       *conf.Remote
}

func New(boot *bootstrap.Bootstrap) (*Assembly, error) {
	config := &conf.Remote{}
	err := boot.App.Config().Read(config)
	if err != nil {
		return nil, errors.WithMessage(err, "read remote config")
	}

	director := service.NewDirector(boot.App.Logger())
	return &Assembly{
		director:   director,
		boot:       boot,
		httpServer: http.NewServer(boot.App.Logger()),
		grpcServer: NewGrpcProxyServer(director),
		logger:     boot.App.Logger(),
		conf:       config,
	}, nil
}

func (a *Assembly) ReceiveConfig(_ context.Context, _ []byte) error {
	return nil
}

func (a *Assembly) ReceiveRoutes(_ context.Context, routes cluster.RoutingConfig) error {
	mux := mux2.NewRouter()
	mux.HandleFunc("/", a.director.Handle)

	a.director.Upgrade(routes)

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
			return a.grpcServer.Serve(lis)
		}),
		app.RunnerFunc(func(ctx context.Context) error {
			return a.httpServer.ListenAndServe(net.JoinHostPort(a.conf.HttpServerHost, a.conf.HttpServerPort))
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
			a.grpcServer.GracefulStop()
			return nil
		}),
		app.CloserFunc(func() error {
			return a.httpServer.Shutdown(context.Background())
		}),
	}
}

func NewGrpcProxyServer(director proxy.StreamDirector) *grpc.Server {
	return grpc.NewServer(
		grpc.CustomCodec(proxy.Codec()), //nolint:staticcheck
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director)),
		grpc.MaxRecvMsgSize(service.MaxMessageSize),
		grpc.MaxSendMsgSize(service.MaxMessageSize),
	)
}

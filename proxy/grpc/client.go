package grpc

import (
	"github.com/integration-system/isp-lib/v2/backend"
	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
	"isp-routing-service/conf"
	"isp-routing-service/domain"
	"isp-routing-service/proxy/grpc/handlers"
)

type grpcProxy struct {
	client *backend.RxGrpcClient
}

func NewProxy() *grpcProxy {
	return &grpcProxy{
		client: backend.NewRxGrpcClient(
			backend.WithDialOptions(
				grpc.WithInsecure(),
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(conf.DefaultMaxResponseBodySize))),
				grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(int(conf.DefaultMaxResponseBodySize))),
			)),
	}
}

func (p *grpcProxy) ProxyRequest(ctx *fasthttp.RequestCtx, path string) domain.ProxyResponse {
	return handlers.Handler.Get(ctx).Complete(ctx, path, p.client)
}

func (p *grpcProxy) Consumer(addr []structure.AddressConfiguration) bool {
	return p.client.ReceiveAddressList(addr)
}

func (p *grpcProxy) Close() {
	_ = p.client.Close()
}

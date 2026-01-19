package middleware

import (
	"context"
	"isp-routing-service/domain"
	"net/http"

	http2 "github.com/txix-open/isp-kit/http"

	"github.com/txix-open/isp-kit/log"
)

type Proxy interface {
	Handle(ctx *domain.RequestContext) error
	PrepareEndpoint(endpoint string, proxyApiPrefix bool) string
}

func Entrypoint(
	maxMessageSize int64,
	proxyApiPrefix bool,
	proxy Proxy,
	logger log.Logger,
) http2.HandlerFunc {
	return http2.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		r.Body = http.MaxBytesReader(w, r.Body, maxMessageSize)

		endpoint := proxy.PrepareEndpoint(r.URL.Path, proxyApiPrefix)
		reqCtx := domain.NewContext(r, w, endpoint)
		reqCtx.SetContext(ctx)

		return proxy.Handle(reqCtx)
	})
}

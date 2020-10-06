package health_check

import (
	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/valyala/fasthttp"
	"isp-routing-service/domain"
)

type healthCheckProxy struct {
	skipAuth       bool
	skipExistCheck bool
}

func NewProxy(skipAuth, skipExistCheck bool) *healthCheckProxy {
	return &healthCheckProxy{skipAuth: skipAuth, skipExistCheck: skipExistCheck}
}

func (p *healthCheckProxy) Consumer(addressList []structure.AddressConfiguration) bool {
	return true
}

func (p *healthCheckProxy) ProxyRequest(ctx *fasthttp.RequestCtx, path string) domain.ProxyResponse {
	ctx.Response.SetBody(ctx.Request.Body())
	ctx.Request.SetRequestURI(path)
	return domain.Create()
}

func (p *healthCheckProxy) SkipAuth() bool {
	return p.skipAuth
}

func (p *healthCheckProxy) SkipExistCheck() bool {
	return p.skipExistCheck
}

func (p *healthCheckProxy) Close() {

}

package health_check

import (
	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/valyala/fasthttp"
	"isp-routing-service/domain"
)

type healthCheckProxy struct {
}

func NewProxy() *healthCheckProxy {
	return &healthCheckProxy{}
}

func (p *healthCheckProxy) Consumer(addressList []structure.AddressConfiguration) bool {
	return true
}

func (p *healthCheckProxy) ProxyRequest(ctx *fasthttp.RequestCtx, path string) domain.ProxyResponse {
	ctx.Response.SetBody(ctx.Request.Body())
	ctx.Request.SetRequestURI(path)
	return domain.Create()
}

func (p *healthCheckProxy) Close() {

}

package http

import (
	"errors"
	"net"
	"strings"

	"github.com/integration-system/isp-lib/v2/structure"
	log "github.com/integration-system/isp-log"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc/codes"
	"isp-routing-service/domain"
	"isp-routing-service/log_code"
	"isp-routing-service/utils"
)

var (
	errNotInitialized = errors.New("http proxy not initialized")
)

type httpProxy struct {
	client         *fasthttp.HostClient
	skipAuth       bool
	skipExistCheck bool
}

func NewProxy(skipAuth, skipExistCheck bool) *httpProxy {
	return &httpProxy{client: nil, skipAuth: skipAuth, skipExistCheck: skipExistCheck}
}

func (p *httpProxy) Consumer(addressList []structure.AddressConfiguration) bool {
	if len(addressList) == 0 {
		p.client = nil
		return true
	}
	addresses := make([]string, len(addressList))
	for key, addr := range addressList {
		addresses[key] = addr.GetAddress()
	}

	p.client = &fasthttp.HostClient{
		Addr: strings.Join(addresses, `,`),
	}
	return true
}

func (p *httpProxy) ProxyRequest(ctx *fasthttp.RequestCtx, path string) domain.ProxyResponse {
	client := p.client
	if client == nil {
		log.Error(log_code.ErrorClientHttp, errNotInitialized)
		utils.WriteError(ctx, errNotInitialized.Error(), codes.Internal, nil)
		return domain.Create().
			SetRequestBody(ctx.Request.Body()).
			SetResponseBody(ctx.Response.Body()).
			SetError(errNotInitialized)
	}

	req := &ctx.Request
	res := &ctx.Response
	req.URI().SetPath("/" + path)

	if addr, _, err := net.SplitHostPort(ctx.RemoteAddr().String()); err == nil {
		req.Header.Add("X-Forwarded-For", addr)
	}

	err := client.Do(req, res)
	if err != nil {
		log.Error(log_code.ErrorClientHttp, err)
	}
	return domain.Create().
		SetRequestBody(ctx.Request.Body()).
		SetResponseBody(ctx.Response.Body()).
		SetError(err)
}

func (p *httpProxy) SkipAuth() bool {
	return p.skipAuth
}

func (p *httpProxy) SkipExistCheck() bool {
	return p.skipExistCheck
}

func (p *httpProxy) Close() {
	p.client = nil
}

package handler

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc/codes"
	"isp-routing-service/domain"
	"isp-routing-service/proxy"
	"isp-routing-service/utils"
)

var (
	helper handlerHelper
)

type handlerHelper struct{}

func CompleteRequest(ctx *fasthttp.RequestCtx) {
	initialPath := string(ctx.Path())

	p, path := proxy.Find(initialPath)
	if p == nil {
		msg := fmt.Sprintf("unknown proxy for '%s'", initialPath)
		utils.WriteError(ctx, msg, codes.NotFound, nil)
		domain.Create().SetError(errors.New(msg))
	}
	p.ProxyRequest(ctx, path)
}

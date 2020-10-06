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
	method, resp := helper.AuthenticateAccountingProxy(ctx)
	statusCode := ctx.Response.StatusCode()
	a, b, c := resp.Get()
	fmt.Println(a, b, c, method, statusCode)

	//logEnable := config.GetRemote().(*conf.RemoteConfig).JournalSetting.Journal.Enable
	////nolint
	//if logEnable && matcher.JournalMethods.Match(method) {
	//	requestBody, responseBody, err := resp.Get()
	//	if err != nil {
	//		if err := invoker.Journal.Error(method, requestBody, responseBody, err); err != nil {
	//			log.Warnf(log_code.WarnJournalCouldNotWriteToFile, "could not write to file journal: %v", err)
	//		}
	//	} else {
	//		if err := invoker.Journal.Info(method, requestBody, responseBody); err != nil {
	//			log.Warnf(log_code.WarnJournalCouldNotWriteToFile, "could not write to file journal: %v", err)
	//		}
	//	}
	//}
}

func (handlerHelper) AuthenticateAccountingProxy(ctx *fasthttp.RequestCtx) (string, domain.ProxyResponse) {
	initialPath := string(ctx.Path())

	p, path := proxy.Find(initialPath)
	if p == nil {
		msg := fmt.Sprintf("unknown proxy for '%s'", initialPath)
		utils.WriteError(ctx, msg, codes.NotFound, nil)
		return initialPath, domain.Create().SetError(errors.New(msg))
	}

	return path, p.ProxyRequest(ctx, path)
}

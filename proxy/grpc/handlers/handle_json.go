package handlers

import (
	"github.com/integration-system/isp-lib/v2/backend"
	"github.com/integration-system/isp-lib/v2/config"
	"github.com/integration-system/isp-lib/v2/isp"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"isp-routing-service/conf"
	"isp-routing-service/domain"
	"isp-routing-service/log_code"
	"isp-routing-service/utils"
)

var handleJson handleJsonDesc

type handleJsonDesc struct{}

func (p handleJsonDesc) Complete(c *fasthttp.RequestCtx, method string, client *backend.RxGrpcClient) domain.ProxyResponse {
	body := c.Request.Body()

	md, methodName := makeMetadata(&c.Request.Header, method)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	grpcSetting := config.GetRemote().(*conf.RemoteConfig).GrpcSetting
	ctx, cancel := context.WithTimeout(ctx, grpcSetting.GetSyncInvokeTimeout())
	defer cancel()

	cli := client.Conn()

	msg, invokerErr := cli.Request(
		ctx,
		&isp.Message{
			Body: &isp.Message_BytesBody{BytesBody: body},
		},
	)

	if response, status, err := getResponse(msg, invokerErr); err == nil {
		c.SetStatusCode(status)
		c.SetContentTypeBytes(utils.JsonContentType)
		c.Response.Header.SetContentLength(len(response))
		_, _ = c.Write(response)
		return domain.Create().SetRequestBody(body).SetResponseBody(c.Response.Body()).SetError(invokerErr)
	} else {
		logHandlerError(log_code.TypeData.JsonContent, methodName, err)
		utils.WriteError(c, errorMsgInternal, codes.Internal, nil)
		return domain.Create().SetRequestBody(body).SetResponseBody(c.Response.Body()).SetError(err)
	}
}

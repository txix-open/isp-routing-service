package handlers

import (
	"fmt"
	"io"
	"strconv"

	"github.com/integration-system/isp-lib/v2/backend"
	"github.com/integration-system/isp-lib/v2/config"
	"github.com/integration-system/isp-lib/v2/isp"
	s "github.com/integration-system/isp-lib/v2/streaming"
	u "github.com/integration-system/isp-lib/v2/utils"
	log "github.com/integration-system/isp-log"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc/codes"
	"isp-routing-service/conf"
	"isp-routing-service/domain"
	"isp-routing-service/log_code"
	"isp-routing-service/utils"
)

var (
	errInvalidJson = errors.New("invalid json format. Expected object or array")
	getFile        getFileDesc
)

type getFileDesc struct{}

func (g getFileDesc) Complete(ctx *fasthttp.RequestCtx, method string, client *backend.RxGrpcClient) domain.ProxyResponse {
	req, err := g.readJsonBody(ctx)
	if err != nil {
		return g.formError(ctx, err, method, err.Error(), codes.InvalidArgument)
	}

	timeout := config.GetRemote().(*conf.RemoteConfig).GrpcSetting.GetStreamInvokeTimeout()
	stream, cancel, err := openStream(&ctx.Request.Header, method, timeout, client)
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()
	if err != nil {
		return g.formError(ctx, err, method, errorMsgInternal, codes.Internal)
	}

	if req != nil {
		value := u.ConvertInterfaceToGrpcStruct(req)
		err := stream.Send(backend.WrapBody(value))
		if err != nil {
			return g.formError(ctx, err, method, errorMsgInternal, codes.Internal)
		}
	}

	msg, err := stream.Recv()
	if err != nil {
		response, status, err := getResponse(nil, err)
		if err == nil {
			ctx.SetStatusCode(status)
			ctx.SetBody(response)
		}
		return domain.Create().SetError(err)
	}

	bf := s.BeginFile{}
	err = bf.FromMessage(msg)
	if err != nil {
		response, status, err := getResponse(nil, err)
		if err == nil {
			ctx.SetStatusCode(status)
			ctx.SetBody(response)
		}
		return domain.Create().SetError(err)
	}
	header := &ctx.Response.Header
	header.Set(headerKeyContentDisposition, fmt.Sprintf("attachment; filename=%s", bf.FileName))
	header.Set(headerKeyContentType, bf.ContentType)
	if bf.ContentLength > 0 {
		header.Set(headerKeyContentLength, strconv.Itoa(int(bf.ContentLength)))
	} else {
		header.Set(headerKeyTransferEncoding, "chunked")
	}

	err = g.write(ctx, stream, method)
	return domain.Create().SetError(err)
}

//nolint
func (getFileDesc) write(ctx *fasthttp.RequestCtx, stream isp.BackendService_RequestStreamClient, method string) error {
	for {
		msg, err := stream.Recv()
		if s.IsEndOfFile(msg) || err == io.EOF {
			return nil
		}
		if err != nil {
			logHandlerError(log_code.TypeData.GetFile, method, err)
			return err
		}
		bytes := msg.GetBytesBody()
		if bytes == nil {
			log.WithMetadata(map[string]interface{}{
				log_code.MdTypeData: log_code.TypeData.GetFile,
				log_code.MdMethod:   method,
			}).Errorf(log_code.WarnProxyGrpcHandler, "Method %s. Expected bytes array", method)
			return nil
		}
		_, err = ctx.Write(bytes)
		if err != nil {
			logHandlerError(log_code.TypeData.GetFile, method, err)
			return err
		}
	}
}

func (getFileDesc) readJsonBody(ctx *fasthttp.RequestCtx) (interface{}, error) {
	requestBody := ctx.Request.Body()
	var body interface{}
	if len(requestBody) == 0 {
		requestBody = []byte("{}")
	}
	switch requestBody[0] {
	case '{':
		body = make(map[string]interface{})
	case '[':
		body = make([]interface{}, 0)
	default:
		return nil, errInvalidJson
	}
	err := json.Unmarshal(requestBody, &body)
	if err != nil {
		return nil, err
	}
	return body, err
}

func (getFileDesc) formError(ctx *fasthttp.RequestCtx, err error, method, msg string, code codes.Code) domain.ProxyResponse {
	logHandlerError(log_code.TypeData.GetFile, method, err)
	utils.WriteError(ctx, msg, code, nil)
	return domain.Create().SetError(err)
}

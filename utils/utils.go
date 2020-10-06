package utils

import (
	"encoding/json"

	"github.com/integration-system/isp-lib/v2/http"
	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc/codes"
)

var (
	JsonContentType = []byte("application/json; charset=utf-8")
)

func WriteError(ctx *fasthttp.RequestCtx, message string, code codes.Code, details []interface{}) {
	grpcCode := code.String()

	structureError := structure.GrpcError{
		ErrorMessage: message,
		ErrorCode:    grpcCode,
		Details:      details,
	}

	ctx.SetContentTypeBytes(JsonContentType)
	ctx.SetStatusCode(http.CodeToHttpStatus(code))
	msg, _ := json.Marshal(structureError)
	ctx.Response.Header.SetContentLength(len(msg))
	_, _ = ctx.Write(msg)
}

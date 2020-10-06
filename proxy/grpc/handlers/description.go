package handlers

import (
	"mime"

	"github.com/integration-system/isp-lib/v2/backend"
	u "github.com/integration-system/isp-lib/v2/utils"
	"github.com/valyala/fasthttp"
	"isp-routing-service/domain"
	"isp-routing-service/utils"
)

var Handler handlerHelper

type (
	handlerHelper struct{}

	handler interface {
		Complete(ctx *fasthttp.RequestCtx, method string, client *backend.RxGrpcClient) domain.ProxyResponse
	}
)

func (h handlerHelper) Get(ctx *fasthttp.RequestCtx) handler {
	isMultipart := h.isMultipart(ctx)
	isExpectFile := string(ctx.Request.Header.Peek(u.ExpectFileHeader)) == "true"

	if isMultipart {
		ctx.Response.Header.SetContentTypeBytes(utils.JsonContentType)
		return sendMultipartData
	}
	if isExpectFile {
		return getFile
	}
	ctx.Response.Header.SetContentTypeBytes(utils.JsonContentType)
	return handleJson
}

func (h handlerHelper) isMultipart(ctx *fasthttp.RequestCtx) bool {
	if !ctx.IsPost() {
		return false
	}
	v := string(ctx.Request.Header.ContentType())
	if v == "" {
		return false
	}
	d, params, err := mime.ParseMediaType(v)
	if err != nil || d != "multipart/form-data" {
		return false
	}
	_, ok := params["boundary"]
	return ok
}

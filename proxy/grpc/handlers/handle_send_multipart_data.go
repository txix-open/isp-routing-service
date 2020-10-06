package handlers

import (
	"mime/multipart"
	"strings"

	"github.com/integration-system/isp-lib/v2/backend"
	"github.com/integration-system/isp-lib/v2/config"
	"github.com/integration-system/isp-lib/v2/isp"
	s "github.com/integration-system/isp-lib/v2/streaming"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc/codes"
	"isp-routing-service/conf"
	"isp-routing-service/domain"
	"isp-routing-service/log_code"
	"isp-routing-service/utils"
)

var sendMultipartData sendMultipartDataDesc

type sendMultipartDataDesc struct{}

func (h sendMultipartDataDesc) Complete(c *fasthttp.RequestCtx, method string, client *backend.RxGrpcClient) domain.ProxyResponse {
	cfg := config.GetRemote().(*conf.RemoteConfig).GrpcSetting
	timeout := cfg.GetStreamInvokeTimeout()

	stream, cancel, err := openStream(&c.Request.Header, method, timeout, client)
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()
	if err != nil {
		logHandlerError(log_code.TypeData.SendMultipart, method, err)
		utils.WriteError(c, errorMsgInternal, codes.Internal, nil)
		return domain.Create().SetError(err)
	}

	form, err := c.MultipartForm()
	defer func() {
		if form != nil {
			_ = form.RemoveAll()
		}
	}()

	if err != nil {
		logHandlerError(log_code.TypeData.SendMultipart, method, err)
		utils.WriteError(c, errorMsgInvalidArg, codes.InvalidArgument, nil)
		return domain.Create().SetError(err)
	}

	resp, proxyResp, ok := h.formResponse(c, stream, form, method)
	err = stream.CloseSend()
	if err != nil {
		logHandlerError(log_code.TypeData.SendMultipart, method, err)
	}

	if ok {
		arrayBody := strings.Join(resp, ",")
		_, err = c.WriteString("[" + arrayBody + "]")
		if err != nil {
			logHandlerError(log_code.TypeData.SendMultipart, method, err)
		}
	}
	return proxyResp
}

func (h sendMultipartDataDesc) formResponse(c *fasthttp.RequestCtx, stream isp.BackendService_RequestStreamClient,
	form *multipart.Form, method string) ([]string, domain.ProxyResponse, bool) {
	formData := make(map[string]interface{}, len(form.Value))
	for k, v := range form.Value {
		if len(v) > 0 {
			formData[k] = v[0]
		}
	}

	var (
		resp      = make([]string, 0)
		buffer    = make([]byte, config.GetRemote().(*conf.RemoteConfig).GrpcSetting.GetTransferFileBufferSize())
		proxyResp domain.ProxyResponse
		ok        = true
		eof       bool
		err       error
	)
	for formDataName, files := range form.File {
		if len(files) == 0 {
			continue
		}
		file := files[0]
		fileName := file.Filename
		contentType := file.Header.Get(headerKeyContentType)
		contentLength := file.Size
		bf := s.BeginFile{
			FileName:      fileName,
			FormDataName:  formDataName,
			ContentType:   contentType,
			ContentLength: contentLength,
			FormData:      formData,
		}
		err = stream.Send(bf.ToMessage())
		ok, eof, proxyResp = checkError(err, c, method)
		if !ok || eof {
			break
		}

		f, err := file.Open()
		ok, eof, proxyResp = checkError(err, c, method)
		if !ok || eof {
			break
		}

		ok, eof, proxyResp = h.transferFile(f, stream, buffer, c, method)
		if !ok || eof {
			break
		}

		msg, invokeErr := stream.Recv()
		ok, eof, proxyResp = checkError(invokeErr, c, method)
		if !ok || eof {
			break
		}

		response, _, err := getResponse(msg, invokeErr)
		if err != nil {
			ok = false
			break
		}
		resp = append(resp, string(response))
	}
	return resp, proxyResp, ok
}

func (sendMultipartDataDesc) transferFile(f multipart.File, stream isp.BackendService_RequestStreamClient,
	buffer []byte, ctx *fasthttp.RequestCtx, method string) (bool, bool, domain.ProxyResponse) {
	var (
		ok        = true
		eof       bool
		proxyResp domain.ProxyResponse
	)
	for {
		n, err := f.Read(buffer)
		if n > 0 {
			err = stream.Send(&isp.Message{Body: &isp.Message_BytesBody{BytesBody: buffer[:n]}})
			ok, eof, proxyResp = checkError(err, ctx, method)
			if !ok || eof {
				break
			}
		}
		if err != nil {
			ok, eof, proxyResp = checkError(err, ctx, method)
			if ok && eof {
				err = stream.Send(s.FileEnd())
				ok, eof, proxyResp = checkError(err, ctx, method)
			}
			break
		}
	}
	return ok, eof, proxyResp
}

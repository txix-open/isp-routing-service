// nolint:wrapcheck
package proxy

import (
	"io"
	"isp-routing-service/domain"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/txix-open/isp-kit/grpc"
	"github.com/txix-open/isp-kit/grpc/client"
	"github.com/txix-open/isp-kit/grpc/isp"
	"github.com/txix-open/isp-kit/json"
	"github.com/txix-open/isp-kit/requestid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// nolint:gochecknoinits
func init() {
	for httpCode, grpcCode := range codeMap {
		inverseCodeMap[grpcCode] = httpCode
	}
}

// nolint:gochecknoglobals
var (
	codeMap = map[int]codes.Code{
		http.StatusOK:                  codes.OK,
		http.StatusRequestTimeout:      codes.Canceled,
		http.StatusBadRequest:          codes.InvalidArgument,
		http.StatusGatewayTimeout:      codes.DeadlineExceeded,
		http.StatusNotFound:            codes.NotFound,
		http.StatusConflict:            codes.AlreadyExists,
		http.StatusForbidden:           codes.PermissionDenied,
		http.StatusUnauthorized:        codes.Unauthenticated,
		http.StatusTooManyRequests:     codes.ResourceExhausted,
		http.StatusPreconditionFailed:  codes.FailedPrecondition,
		http.StatusNotImplemented:      codes.Unimplemented,
		http.StatusInternalServerError: codes.Internal,
		http.StatusServiceUnavailable:  codes.Unavailable,
	}
	inverseCodeMap = map[codes.Code]int{}
)

type Grpc struct {
	cli *client.Client
}

func NewGrpc(cli *client.Client) Grpc {
	return Grpc{
		cli: cli,
	}
}

func (p Grpc) PrepareEndpoint(endpoint string, proxyApiPrefix bool) string {
	endpoint = strings.TrimPrefix(endpoint, "/")
	if proxyApiPrefix {
		return endpoint
	}

	// api/endpoint -> endpoint
	return strings.TrimPrefix(endpoint, domain.GrpcApiPrefix)
}

func (p Grpc) Handle(ctx *domain.RequestContext) error {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return errors.WithMessage(err, "grpc: read body")
	}

	requestId := requestid.FromContext(ctx.Context())

	md := httpHeadersToGrpcMetadata(ctx.Request().Header)
	md.Set(grpc.ProxyMethodNameHeader, ctx.Endpoint())

	requestContext := metadata.NewOutgoingContext(ctx.Context(), md)

	result, err := p.cli.BackendClient().Request(requestContext, &isp.Message{
		Body: &isp.Message_BytesBody{BytesBody: body},
	})
	if err != nil {
		return p.writeGrpcError(err, ctx.ResponseWriter(), ctx.Endpoint(), requestId)
	}

	return p.writeResponse(http.StatusOK, result.GetBytesBody(), requestId, ctx.ResponseWriter())
}

func (p Grpc) Close() error {
	return p.cli.Close()
}

func (p Grpc) writeGrpcError(err error, w http.ResponseWriter, endpoint string, requestId string) error {
	status, ok := status.FromError(err)
	if !ok {
		return domain.NewHttpError(
			http.StatusServiceUnavailable,
			"upstream is not available",
			errors.WithMessage(err, "grpc proxy"),
		)
	}

	statusCode := p.codeToHttpStatus(status.Code())
	for _, detail := range status.Details() {
		switch typeOfDetail := detail.(type) {
		case *isp.Message:
			switch {
			case typeOfDetail.GetBytesBody() != nil:
				return p.writeResponse(statusCode, typeOfDetail.GetBytesBody(), requestId, w)
			case typeOfDetail.GetListBody() != nil:
				return p.writeProto(statusCode, typeOfDetail.GetListBody(), requestId, w)
			case typeOfDetail.GetStructBody() != nil:
				return p.writeProto(statusCode, typeOfDetail.GetStructBody(), requestId, w)
			}
		default:
			return p.writeProto(statusCode, typeOfDetail, requestId, w)
		}
	}

	return domain.NewHttpError(
		statusCode,
		status.Message(),
		errors.WithMessagef(err, "proxy '%s'", endpoint),
	)
}

func (p Grpc) writeProto(statusCode int, proto any, requestId string, w http.ResponseWriter) error {
	data, err := json.Marshal(proto)
	if err != nil {
		return errors.WithMessage(err, "marshal grpc details to json")
	}
	return p.writeResponse(statusCode, data, requestId, w)
}

func (p Grpc) writeResponse(statusCode int, data []byte, requestId string, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if requestId != "" {
		w.Header().Set(requestid.Header, requestId)
	}
	w.WriteHeader(statusCode)
	_, err := w.Write(data)
	if err != nil {
		return errors.WithMessage(err, "response write")
	}
	return nil
}

func (p Grpc) codeToHttpStatus(code codes.Code) int {
	s, ok := inverseCodeMap[code]
	if !ok {
		return http.StatusInternalServerError
	}

	return s
}

func httpHeadersToGrpcMetadata(h http.Header) metadata.MD {
	md := metadata.MD{}
	for header, values := range h {
		header = strings.ToLower(header)
		if strings.HasPrefix(header, "x-") {
			md.Set(header, values...)
		}
	}
	return md
}

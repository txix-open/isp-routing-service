package middleware

import (
	"context"
	"net/http"

	"isp-routing-service/domain"

	http2 "github.com/txix-open/isp-kit/http"
	"github.com/txix-open/isp-kit/log"
)

type HttpError interface {
	WriteError(w http.ResponseWriter) error
}

func ErrorHandler(logger log.Logger) http2.Middleware {
	return func(next http2.HandlerFunc) http2.HandlerFunc {
		return http2.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			err := next(ctx, w, r)
			if err == nil {
				return nil
			}

			logger.Error(ctx, err)

			httpErr, ok := err.(HttpError)
			if ok {
				return httpErr.WriteError(w)
			}

			return domain.NewHttpError(http.StatusInternalServerError, "internal service error", err).
				WriteError(w)
		})
	}
}

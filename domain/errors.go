package domain

import (
	"net/http"

	"github.com/txix-open/isp-kit/json"
)

type HttpError struct {
	statusCode  int
	userMessage string
	details     []any
	err         error
}

func NewHttpError(statusCode int, userMessage string, internalError error) *HttpError {
	return &HttpError{
		statusCode:  statusCode,
		userMessage: userMessage,
		err:         internalError,
	}
}

func (e *HttpError) Error() string {
	return e.err.Error()
}

func (e *HttpError) WriteError(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.statusCode)
	data := map[string]any{
		"errorCode":    http.StatusText(e.statusCode),
		"errorMessage": e.userMessage,
		"details":      e.details,
	}
	return json.NewEncoder(w).Encode(data) // nolint:wrapcheck
}

func (e *HttpError) WithDetails(details ...any) {
	e.details = details
}

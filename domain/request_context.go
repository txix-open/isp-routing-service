package domain

import (
	"context"
	"net/http"
	"strings"
)

type RequestContext struct {
	request        *http.Request
	responseWriter http.ResponseWriter

	endpoint string

	queryParams map[string]string
}

func NewContext(request *http.Request, response http.ResponseWriter, endpoint string) *RequestContext {
	return &RequestContext{
		request:        request,
		responseWriter: response,
		endpoint:       endpoint,
	}
}

func (c *RequestContext) Request() *http.Request {
	return c.request
}

func (c *RequestContext) ResponseWriter() http.ResponseWriter {
	return c.responseWriter
}

func (c *RequestContext) SetResponseWriter(writer http.ResponseWriter) {
	c.responseWriter = writer
}

func (c *RequestContext) Endpoint() string {
	return c.endpoint
}

func (c *RequestContext) Context() context.Context {
	return c.request.Context()
}

func (c *RequestContext) SetContext(ctx context.Context) {
	c.request = c.request.WithContext(ctx)
}

func (c *RequestContext) Param(name string) string {
	value := c.request.Header.Get(name)
	if value != "" {
		return strings.TrimSpace(value)
	}

	if c.queryParams == nil {
		query := c.request.URL.Query()
		c.queryParams = map[string]string{}
		for key, values := range query {
			if len(values) == 0 {
				continue
			}
			c.queryParams[strings.ToLower(key)] = values[0]
		}
	}
	value = c.queryParams[strings.ToLower(name)]

	return strings.TrimSpace(value)
}

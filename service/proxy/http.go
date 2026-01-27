package proxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"isp-routing-service/domain"

	"github.com/pkg/errors"
	"github.com/txix-open/isp-kit/lb"
)

// nolint:gochecknoglobals,mnd
var (
	httpTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: defaultTransportDialContext(&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 90 * time.Second,
		}),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1024,
		MaxIdleConnsPerHost:   256,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
)

type Http struct {
	hostManager *lb.RoundRobin
	proxies     map[string]*httputil.ReverseProxy
}

func NewHttp(addresses []string) Http {
	p := Http{
		hostManager: lb.NewRoundRobin(addresses),
		proxies:     make(map[string]*httputil.ReverseProxy),
	}

	for _, addr := range addresses {
		target, _ := url.Parse("http://" + addr)
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = httpTransport
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			_ = domain.NewHttpError(
				http.StatusServiceUnavailable,
				"upstream is not available",
				errors.WithMessagef(err, "http proxy to %s", addr),
			).WriteError(w)
		}
		p.proxies[addr] = proxy
	}

	return p
}

func (p Http) PrepareEndpoint(endpoint string, proxyApiPrefix bool) string {
	if proxyApiPrefix {
		return endpoint
	}
	return strings.TrimPrefix(endpoint, domain.ApiPrefix)
}

func (p Http) Handle(ctx *domain.RequestContext) error {
	host, err := p.hostManager.Next()
	if err != nil {
		return errors.WithMessage(err, "http: next host")
	}

	proxy := p.proxies[host]

	req := ctx.Request()
	req.URL.Scheme = "http"
	req.URL.Host = host
	req.Host = host
	req.URL.Path = ctx.Endpoint()

	proxy.ServeHTTP(ctx.ResponseWriter(), req)

	return nil
}

func (p Http) Close() error {
	return nil
}

func defaultTransportDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return dialer.DialContext
}

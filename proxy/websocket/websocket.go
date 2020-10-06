package websocket

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/integration-system/isp-lib/v2/structure"
	log "github.com/integration-system/isp-log"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc/codes"
	"isp-routing-service/domain"
	"isp-routing-service/log_code"
	"isp-routing-service/utils"
)

const (
	writeBufSize = 4 << 10
	readBufSize  = 4 << 10
)

var (
	errNoAddresses = errors.New("no available address")
	pool           = &sync.Pool{New: func() interface{} {
		buf := make([]byte, readBufSize)
		return &buf
	}}
)

// used to filter client request headers
//
var forbiddenDuplicateHeaders = map[string]struct{}{
	"Upgrade":                  {},
	"Connection":               {},
	"Sec-Websocket-Key":        {},
	"Sec-Websocket-Version":    {},
	"Sec-Websocket-Extensions": {},
	"Sec-Websocket-Protocol":   {},
}

var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  readBufSize,
	WriteBufferSize: writeBufSize,
	CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
		return true
	},
}

type websocketProxy struct {
	addrs          *RoundRobinAddrs
	skipAuth       bool
	skipExistCheck bool
}

func NewProxy(skipAuth, skipExistCheck bool) *websocketProxy {
	return &websocketProxy{addrs: nil, skipAuth: skipAuth, skipExistCheck: skipExistCheck}
}

func (p *websocketProxy) Consumer(addressList []structure.AddressConfiguration) bool {
	if len(addressList) == 0 {
		p.addrs = nil
	} else {
		p.addrs = NewRoundRobinAddrs(addressList)
	}
	return true
}

func (p *websocketProxy) ProxyRequest(ctx *fasthttp.RequestCtx, path string) domain.ProxyResponse {
	addrs := p.addrs
	if addrs == nil {
		msg := errNoAddresses
		log.Error(log_code.ErrorWebsocketProxy, msg)
		utils.WriteError(ctx, msg.Error(), codes.Internal, nil)
		return domain.Create().
			SetRequestBody(ctx.Request.Body()).
			SetResponseBody(ctx.Response.Body()).
			SetError(errNoAddresses)
	}

	reqHeaders := fasthttp.RequestHeader{}
	ctx.Request.Header.CopyTo(&reqHeaders)
	reqHost := string(ctx.Request.URI().Host())
	reqScheme := string(ctx.Request.URI().Scheme())

	if addr, _, err := net.SplitHostPort(ctx.RemoteAddr().String()); err == nil {
		reqHeaders.Add("X-Forwarded-For", addr)
	}

	err := upgrader.Upgrade(ctx, func(incomingConn *websocket.Conn) {
		outgoingDialer := websocket.Dialer{
			ReadBufferSize:  readBufSize,
			WriteBufferSize: writeBufSize,
			NetDial:         net.Dial,
		}

		addr := addrs.Get()
		uri := ctx.Request.URI()
		uri.SetSchemeBytes([]byte("ws"))
		uri.SetHost(addr.GetAddress())
		uri.SetPath("/" + path)
		header := http.Header{}

		reqHeaders.VisitAll(func(key, value []byte) {
			keyStr := string(key)
			if _, forbidden := forbiddenDuplicateHeaders[keyStr]; !forbidden {
				header.Add(keyStr, string(value))
			}
		})

		cookie := make([]*http.Cookie, 0)
		reqHeaders.VisitAllCookie(func(key, value []byte) {
			cookie = append(cookie, &http.Cookie{Domain: reqHost, Name: string(key), Value: string(value)})
		})
		cookies := &cookiejar.Jar{}
		cookies.SetCookies(&url.URL{Host: reqHost, Scheme: reqScheme}, cookie)
		outgoingDialer.Jar = cookies

		//nolint
		outgoingConn, _, err := outgoingDialer.Dial(uri.String(), header)
		if err == nil {
			go func() {
				_ = p.proxyConn(outgoingConn, incomingConn)
			}()
			_ = p.proxyConn(incomingConn, outgoingConn)
		} else {
			log.Errorf(log_code.ErrorWebsocketProxy, "unable to connect to service %s: %v", uri.String(), err)
			_ = incomingConn.Close()
			_ = outgoingConn.Close()
		}
	})

	return domain.Create().SetError(err)
}

func (p *websocketProxy) SkipAuth() bool {
	return p.skipAuth
}

func (p *websocketProxy) SkipExistCheck() bool {
	return p.skipExistCheck
}

func (p *websocketProxy) Close() {
}

func (p *websocketProxy) proxyConn(from, to *websocket.Conn) error {
	buf := pool.Get().(*[]byte)
	defer func() {
		_ = from.Close()
		_ = to.Close()
		pool.Put(buf)
	}()
	for {
		msgType, reader, err := from.NextReader()
		if err != nil {
			return err
		}
		writer, err := to.NextWriter(msgType)
		if err != nil {
			return err
		}
		if _, err := io.CopyBuffer(writer, reader, *buf); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
	}
}

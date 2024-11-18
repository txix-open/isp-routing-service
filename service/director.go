package service

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/txix-open/isp-kit/cluster"
	ispgrpc "github.com/txix-open/isp-kit/grpc"
	"github.com/txix-open/isp-kit/lb"
	"github.com/txix-open/isp-kit/log"
	"github.com/vgough/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	connectionTimeout = 1 * time.Second
	MaxMessageSize    = 64 * 1024 * 1024
)

type Director struct {
	addressesToConns     map[string]*Conn
	endpointsToAddresses map[string]*lb.RoundRobin
	lock                 *sync.RWMutex
}

func NewDirector() *Director {
	return &Director{
		addressesToConns:     map[string]*Conn{},
		endpointsToAddresses: map[string]*lb.RoundRobin{},
		lock:                 &sync.RWMutex{},
	}
}

func (d *Director) Upgrade(logger log.Logger, config cluster.RoutingConfig) {
	addressesToConns := make(map[string]*Conn)
	endpointsToAddressArray := make(map[string][]string)
	aliveBackendsCount := 0
	for _, declaration := range config {
		if declaration.Address.IP == "" || declaration.Address.Port == "" || len(declaration.Endpoints) == 0 {
			continue
		}
		addr := net.JoinHostPort(declaration.Address.IP, declaration.Address.Port)

		oldConn, present := d.addressesToConns[addr]
		if present {
			addressesToConns[addr] = oldConn
		} else {
			conn, err := d.dial(addr)
			if err != nil {
				logger.Error(
					context.Background(),
					"couldn't connect",
					log.String("moduleName", declaration.ModuleName),
					log.String("address", addr),
					log.Any("error", err),
				)
				addressesToConns[addr] = NewConn(nil, false)
			} else {
				addressesToConns[addr] = NewConn(conn, true)
				aliveBackendsCount++
			}
		}

		for _, endpoint := range declaration.Endpoints {
			endpointsToAddressArray[endpoint.Path] = append(
				endpointsToAddressArray[endpoint.Path],
				addr,
			)
		}
	}

	endpointsToAddresses := make(map[string]*lb.RoundRobin)
	for endpoint, addresses := range endpointsToAddressArray {
		endpointsToAddresses[endpoint] = lb.NewRoundRobin(addresses)
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	for addr, conn := range d.addressesToConns {
		if _, present := addressesToConns[addr]; !present && conn.alive {
			_ = conn.conn.Close()
		}
	}
	d.addressesToConns = addressesToConns
	d.endpointsToAddresses = endpointsToAddresses

	logger.Info(
		context.Background(),
		"change routing table",
		log.Int("totalBackends", len(d.addressesToConns)),
		log.Int("aliveBackends", aliveBackendsCount),
		log.Int("totalEndpoints", len(d.endpointsToAddresses)),
	)
}

func (d *Director) Connect(ctx context.Context, _ string) (context.Context, *grpc.ClientConn, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, nil, status.Error(codes.DataLoss, "could not read metadata from request context") //nolint:wrapcheck
	}
	endpoint, err := ispgrpc.StringFromMd(ispgrpc.ProxyMethodNameHeader, md)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "get '%s' from metadata", ispgrpc.ProxyMethodNameHeader)
	}

	d.lock.RLock()
	defer d.lock.RUnlock()

	lb, ok := d.endpointsToAddresses[endpoint]
	if !ok {
		return nil, nil, status.Errorf(codes.Unimplemented, "unknown endpoint %s", endpoint)
	}
	addr, err := lb.Next()
	if err != nil {
		return nil, nil, errors.WithMessage(err, "load balancer/next")
	}

	conn, ok := d.addressesToConns[addr]
	if !ok {
		return nil, nil, status.Errorf(codes.Unavailable, "connection not found, addr: %s, endpoint: %s", addr, endpoint)
	}
	if !conn.alive {
		return nil, nil, status.Errorf(codes.Unavailable, "connection is not alive, addr: %s, endpoint: %s", addr, endpoint)
	}

	return ctx, conn.conn, nil
}

func (d *Director) Release(_ context.Context, _ *grpc.ClientConn) {

}

func (d *Director) dial(addr string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	cli, err := grpc.DialContext( //nolint:staticcheck
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), //nolint:staticcheck
		grpc.WithDefaultCallOptions(grpc.ForceCodec(proxy.Codec())),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(MaxMessageSize)),
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "grpc dial to '%s'", addr)
	}
	return cli, nil
}

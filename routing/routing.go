package routing

import (
	"context"
	"sync"
	"time"

	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/integration-system/isp-lib/v2/utils"
	log "github.com/integration-system/isp-log"
	"github.com/vgough/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"isp-routing-service/log_code"
)

const (
	connectionTimeout = 3 * time.Second
	MaxMessageSize    = 64 * 1024 * 1024
)

var (
	connections       = make(map[string]*grpc.ClientConn)
	routingConfigs    = make(map[string]*RoundRobinBalancer)
	routingRawConfigs = structure.RoutingConfig{}
	routingLock       = sync.RWMutex{}

	initializingLock = sync.Mutex{}
	initialized      = false
)

func GetRouter() proxy.StreamDirector {
	return &director{}
}

func GetRoutingConfig() map[string]*RoundRobinBalancer {
	return routingConfigs
}

func GetRoutingRawConfig() structure.RoutingConfig {
	return routingRawConfigs
}

func MarkUninitialized() {
	initializingLock.Lock()
	initialized = false
	initializingLock.Unlock()
}

func InitRoutes(configs structure.RoutingConfig) (bool, bool) {
	initializingLock.Lock()
	defer initializingLock.Unlock()

	newConnections := make(map[string]*grpc.ClientConn)
	newConfig := make(map[string]*RoundRobinBalancer)
	hasErrors := false

	for _, backend := range configs {
		if backend.Address.IP == "" || backend.Address.Port == "" || len(backend.Endpoints) == 0 {
			continue
		}
		addr := backend.Address.GetAddress()

		//initializing new connections may be long time cos try connect with blocking
		if oldConn, present := connections[addr]; !present {
			conn, err := dial(addr)
			if err == nil {
				newConnections[addr] = conn
			} else {
				log.WithMetadata(map[string]interface{}{
					log_code.MdModuleName: backend.ModuleName,
					log_code.MdAddr:       addr,
				}).Errorf(log_code.ErrorNotConnectToModule, "could not connect; error: %v", err)
				hasErrors = true
				continue //do not add methods to routing table
			}
		} else {
			newConnections[addr] = oldConn
		}

		for _, endpoint := range backend.Endpoints {
			balancer, present := newConfig[endpoint.Path]
			if present {
				balancer.addrList = append(balancer.addrList, addr)
				balancer.length++
			} else {
				array := make([]string, 1)
				array[0] = addr
				newConfig[endpoint.Path] = &RoundRobinBalancer{
					addrList: array,
					mu:       sync.Mutex{},
					next:     0,
					length:   1,
				}
			}
		}
	}

	routingLock.Lock()

	//close connections
	for addr, conn := range connections {
		if _, present := newConnections[addr]; !present {
			_ = conn.Close()
		}
	}

	routingRawConfigs = configs
	connections = newConnections
	routingConfigs = newConfig

	routingLock.Unlock()

	firstInit := initialized == false
	initialized = true

	return firstInit, hasErrors
}

func dial(addr string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	return grpc.DialContext(
		dialCtx,
		addr,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.CallCustomCodec(proxy.Codec())),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(MaxMessageSize)),
	)
}

type director struct {
}

func (d director) Connect(ctx context.Context, _ string) (context.Context, *grpc.ClientConn, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, nil, status.Error(codes.DataLoss, "could not read metadata from request")
	}
	methodArray, present := md[utils.ProxyMethodNameHeader]
	if !present || len(methodArray) == 0 {
		return nil, nil, status.Errorf(codes.DataLoss, "metadata [%s] is required", utils.ProxyMethodNameHeader)
	}
	method := methodArray[0]

	routingLock.RLock()
	defer routingLock.RUnlock()

	balancer, conPresent := routingConfigs[method]
	if !conPresent {
		return nil, nil, status.Errorf(codes.Unimplemented, "unknown method %s", method)
	}
	addr := balancer.Next()
	if conn, conPresent := connections[addr]; conPresent {
		return ctx, conn, nil
	} else {
		return nil, nil, status.Errorf(codes.Unavailable, "backend %s unavailable", addr)
	}
}

func (d director) Release(ctx context.Context, conn *grpc.ClientConn) {

}

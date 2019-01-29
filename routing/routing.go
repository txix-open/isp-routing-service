package routing

import (
	"sync"
	"time"

	"isp-routing-service/model"

	"github.com/integration-system/isp-lib/grpc-proxy"
	"github.com/integration-system/isp-lib/logger"
	"github.com/integration-system/isp-lib/structure"
	"github.com/integration-system/isp-lib/utils"
	"github.com/processout/grpc-go-pool"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	initialPoolSize   = 1
	poolCapacity      = 3
	connectionTimeout = 3 * time.Second
)

var (
	connections       = make(map[string]*grpcpool.Pool)
	routingConfigs    = make(map[string]*RoundRobinBalancer)
	routingRawMap     = make(map[string]map[string]structure.EndpointConfig)
	routingRawConfigs = structure.RoutingConfig{}
	routingLock       = sync.RWMutex{}

	initializingLock = sync.Mutex{}
	initialized      = false
)

func GetRouter() grpc_proxy.StreamDirector {
	return director
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

	newRoutingRawMap := make(map[string]map[string]structure.EndpointConfig)
	newConnections := make(map[string]*grpcpool.Pool)
	newConfig := make(map[string]*RoundRobinBalancer)
	hasErrors := false

	for _, backend := range configs {
		if backend.Address.IP == "" || backend.Address.Port == "" || len(backend.Endpoints) == 0 {
			continue
		}
		countEndpoints := 0
		for _, v := range backend.Endpoints {
			if !v.IgnoreOnRouter {
				countEndpoints += 1
			}
		}
		if countEndpoints == 0 {
			continue
		}
		addr := backend.Address.GetAddress()

		//initializing new connections may be long time cos try connect with blocking
		if oldPool, present := connections[addr]; !present {
			pool, err := createConnPool(addr)
			if err == nil {
				newConnections[addr] = pool
			} else {
				logger.Errorf("Could not connect to %s(%s). Error: %v", backend.ModuleName, addr, err)
				hasErrors = true
				continue //do not add methods to routing table
			}
		} else {
			newConnections[addr] = oldPool
		}

		for _, endpoint := range backend.Endpoints {
			if _, ok := newRoutingRawMap[addr]; ok {
				newRoutingRawMap[addr][endpoint.Path] = endpoint
			} else {
				newRoutingRawMap[addr] = map[string]structure.EndpointConfig{endpoint.Path: endpoint}
			}

			ensureHistogramForMethod(endpoint.Path)

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
	for addr, pool := range connections {
		if _, present := newConnections[addr]; !present {
			pool.Close()
		}
	}

	routingRawConfigs = configs
	routingRawMap = newRoutingRawMap
	connections = newConnections
	routingConfigs = newConfig

	routingLock.Unlock()

	firstInit := initialized == false
	initialized = true

	return firstInit, hasErrors
}

func createConnPool(addr string) (*grpcpool.Pool, error) {
	return grpcpool.New(getConnFactory(addr), initialPoolSize, poolCapacity, 30*time.Minute)
}

func getConnFactory(addr string) grpcpool.Factory {
	return func() (*grpc.ClientConn, error) {
		dialCtx, _ := context.WithTimeout(context.Background(), connectionTimeout)
		return grpc.DialContext(
			dialCtx,
			addr,
			grpc.WithInsecure(),
			grpc.WithBlock(),
			grpc.WithDefaultCallOptions(grpc.CallCustomCodec(grpc_proxy.Codec())),
		)
	}
}

func errorHandler(code codes.Code, errorMessage string, args ...interface{}) error {
	incCounter(code.String())
	return status.Errorf(code, errorMessage, args)
}

func director(incomingCtx context.Context, _ string, processor grpc_proxy.RequestProcessor) error {
	executeTimeTimer := mh.StartTotalTimer()

	md, ok := metadata.FromIncomingContext(incomingCtx)
	if !ok {
		return errorHandler(codes.DataLoss, "Could not read metadata from request")
	}
	methodArray, present := md[utils.ProxyMethodNameHeader]
	if !present {
		return errorHandler(codes.DataLoss, "Metadata [%s] is required", utils.ProxyMethodNameHeader)
	}
	method := methodArray[0]

	routingLock.RLock()
	defer routingLock.RUnlock()

	balancer, conPresent := routingConfigs[method]
	if !conPresent {
		return errorHandler(codes.Unimplemented, "Unknown method %s", method)
	}
	addr := balancer.Next()
	if routingRawMap[addr][method].Inner {
		appId, _ := utils.ResolveMetadataIdentity(utils.ApplicationIdHeader, md)
		if appId >= 0 {
			adminToken, present := md[utils.ADMIN_AUTH_HEADER_NAME]
			if !present || len(adminToken) == 0 || adminToken[0] == "" {
				return errorHandler(
					codes.Unauthenticated,
					"Header with admin token [%s] is required",
					utils.ADMIN_AUTH_HEADER_NAME,
				)
			}
			token, err := model.GetToken(adminToken[0])
			if err != nil {
				return errorHandler(codes.Internal, "Error receiving token from bd", err)
			}
			if token == nil {
				return errorHandler(codes.PermissionDenied, "Not allowed to call this method")
			}
		}
	}

	if pool, conPresent := connections[addr]; conPresent {
		con, err := pool.Get(incomingCtx)
		if err != nil {
			s, ok := status.FromError(err)
			if con != nil && (!ok || s.Code() == codes.Unavailable) {
				con.Unhealthy()
			}
			logger.Errorf("Error: %v", err)
			return errorHandler(codes.Unavailable, "Backend %s unavailable", addr)
		}

		currentMethodTimer := startTimer(method)
		outCtx := metadata.NewOutgoingContext(incomingCtx, md.Copy())
		err = processor(outCtx, con.ClientConn)
		con.Close() //put back to pool

		if err == nil {
			currentMethodTimer.End()
			executeTimeTimer.End()
			return nil
		} else {
			if st, ok := status.FromError(err); ok {
				incCounter(st.Code().String())
			}
			return err
		}
	} else {
		return errorHandler(codes.Unavailable, "Backend %s unavailable", addr)
	}
}

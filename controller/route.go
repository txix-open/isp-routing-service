package controller

import (
	"encoding/json"
	"github.com/integration-system/isp-lib/logger"
	"github.com/integration-system/isp-lib/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"isp-routing-service/generated"
	"isp-routing-service/routing"
)

type server struct{}

func GetGRPCServer() server {
	return server{}
}

type RoutingResponse []*generated.BackendConfig

func (s *server) GetRoutes(ctx context.Context, in *generated.Empty) (*generated.RoutingConfig, error) {
	bytes, err := json.Marshal(routing.GetRoutingRawConfig())
	if err != nil {
		return nil, createUnknownError(err)
	}
	var response RoutingResponse
	err = json.Unmarshal(bytes, &response)
	if err != nil {
		return nil, createUnknownError(err)
	}
	return &generated.RoutingConfig{Routes: response}, nil
}

func createUnknownError(err error) error {
	logger.Error(err)
	st := status.New(codes.Unknown, utils.ServiceError)
	return st.Err()
}

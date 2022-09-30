package service

import (
	"google.golang.org/grpc"
)

type Conn struct {
	conn  *grpc.ClientConn
	alive bool
}

func NewConn(clientConn *grpc.ClientConn, alive bool) *Conn {
	return &Conn{
		conn:  clientConn,
		alive: alive,
	}
}

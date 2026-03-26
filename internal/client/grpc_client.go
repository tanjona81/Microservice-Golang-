package client

import (
	"example/hello/internal/domain"

	pb "github.com/tanjona81/gRPC-Golang-/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewUserClient(addr string) (pb.UserServiceClient, *grpc.ClientConn, error) {
	// Non-blocking way to initialize
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, domain.WrapUniversalError(err, "user not found")
	}

	return pb.NewUserServiceClient(conn), conn, nil
}

package main

import (
	"context"
	"fmt"
	"github.com/skonto/test-reverse-proxy/pkg/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
)

type gServer struct {
	pb.GreetingServiceServer
}

func (s *gServer) Greeting(ctx context.Context, req *pb.GreetingServiceRequest) (*pb.GreetingServiceReply, error) {
	return &pb.GreetingServiceReply{
		Message: fmt.Sprintf("Hello, %s", req.Name),
	}, nil
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	pb.RegisterGreetingServiceServer(s, &gServer{})

	reflection.Register(s)

	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

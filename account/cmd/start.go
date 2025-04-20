package main

import (
	"context"
	"log"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmnzx/slopify/account/handler"
	"github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/account/repository"
	"github.com/lmnzx/slopify/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dbpool, err := pgxpool.New(context.Background(), "postgresql://postgres:postgres@localhost:5432/slopify?sslmode=disable")
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer dbpool.Close()

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("unable to ping database: %v", err)
	}

	queries := repository.New(dbpool)

	lis, err := net.Listen("tcp", ":3000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(logger.UnaryServerInterceptor()),
	)

	h := handler.NewGrpcHandler(queries)
	proto.RegisterAccountServiceServer(s, h)
	reflection.Register(s)

	log.Println("server listening at", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

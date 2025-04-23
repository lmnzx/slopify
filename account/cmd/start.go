package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/lmnzx/slopify/account/handler"
	"github.com/lmnzx/slopify/account/repository"
	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	log := middleware.GetLogger()

	dbpool, err := pgxpool.New(context.Background(), "postgresql://postgres:postgres@localhost:5432/slopify?sslmode=disable")
	if err != nil {
		log.Fatal().Err(err).Msg("unable to connect to database")
	}
	defer dbpool.Close()

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("unable to ping database")
	}

	queries := repository.New(dbpool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// auth service
	conn, err := grpc.NewClient(":8000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect")
	}
	defer conn.Close()

	c := auth.NewAuthServiceClient(conn)

	var wg sync.WaitGroup

	wg.Add(1)
	go handler.StartGrpcServer(ctx, queries, &wg)

	wg.Add(1)
	go handler.StartRestServer(ctx, queries, c, &wg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Info().Msg("received shutdown signal")
	cancel()

	wg.Wait()
}

package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/lmnzx/slopify/product/config"
	"github.com/lmnzx/slopify/product/handler"
	"github.com/lmnzx/slopify/product/repository"
	scrips "github.com/lmnzx/slopify/product/scripts"
	"github.com/meilisearch/meilisearch-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	config := config.GetConfig()
	log := middleware.GetLogger()

	dbpool, err := pgxpool.New(context.Background(), config.GetDBConnectionString())
	if err != nil {
		log.Fatal().Err(err).Msg("unable to connect to database")
	}
	defer dbpool.Close()

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("unable to ping database")
	}

	client := meilisearch.New(config.Meilisearch.Url, meilisearch.WithAPIKey(config.Meilisearch.Key))
	defer client.Close()

	index := client.Index("products")

	queries := repository.New(dbpool)

	conn, err := grpc.NewClient(config.AuthServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect")
	}
	defer conn.Close()

	c := auth.NewAuthServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scrips.Seed(queries, index)

	var wg sync.WaitGroup

	wg.Add(1)
	go handler.StartRestServer(ctx, config.RestServerAddress, queries, index, c, &wg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Info().Msg("received shutdown signal")
	cancel()

	wg.Wait()
}

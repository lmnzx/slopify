package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/instrumentation"
	"github.com/lmnzx/slopify/pkg/logger"
	"github.com/lmnzx/slopify/product/config"
	"github.com/lmnzx/slopify/product/handler"
	"github.com/lmnzx/slopify/product/repository"
	scrips "github.com/lmnzx/slopify/product/scripts"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meilisearch/meilisearch-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	config := config.GetConfig()
	log := logger.GetLogger()

	cleanup, err := instrumentation.Init(instrumentation.InstrumentationConfig{
		ServiceName:     config.Name,
		ServiceVersion:  config.Version,
		CollectorURL:    config.OtelCollectorURL,
		MetricsInterval: 10 * time.Second,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize instrumentation")
	}
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbCfg, err := pgxpool.ParseConfig(config.GetDBConnectionString())
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse database connection string")
	}

	dbCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	dbpool, err := pgxpool.NewWithConfig(ctx, dbCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to connect to database")
	}
	defer dbpool.Close()

	if err := dbpool.Ping(ctx); err != nil {
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

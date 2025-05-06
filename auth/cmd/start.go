package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/auth/config"
	"github.com/lmnzx/slopify/auth/handler"
	"github.com/lmnzx/slopify/auth/internal"
	"github.com/lmnzx/slopify/pkg/instrumentation"
	"github.com/lmnzx/slopify/pkg/logger"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeyotel"
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

	client, err := valkey.ParseURL(config.GetDBConnectionString())
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse valkey url")
	}

	valkeyClient, err := valkeyotel.NewClient(client)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to connect to valkey database")
	}
	if err := valkeyClient.Do(ctx, valkeyClient.B().Ping().Build()).Error(); err != nil {
		log.Fatal().Err(err).Msg("unable to ping to valkey database")
	}

	conn, err := grpc.NewClient(config.AccountServiceAddress,
		grpc.WithUnaryInterceptor(instrumentation.UnaryClientInstrumentationMiddleware(config.Name)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect")
	}
	defer conn.Close()

	c := account.NewAccountServiceClient(conn)

	var wg sync.WaitGroup

	secrets := internal.Secrets{
		AccessTokenSecret:  config.Secrets.AccessTokenSecret,
		RefreshTokenSecret: config.Secrets.RefreshTokenSecret,
	}

	wg.Add(1)
	go handler.StartGrpcServer(ctx, config.GrpcServerAddress, valkeyClient, c, secrets, &wg)

	wg.Add(1)
	go handler.StartRestServer(ctx, config.RestServerAddress, valkeyClient, c, secrets, &wg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Info().Msg("received shutdown signal")
	cancel()

	wg.Wait()
}

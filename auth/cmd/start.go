package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/auth/config"
	"github.com/lmnzx/slopify/auth/handler"
	"github.com/lmnzx/slopify/pkg/middleware"

	"github.com/valkey-io/valkey-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	config := config.GetConfig()
	log := middleware.GetLogger()

	client, err := valkey.ParseURL(config.GetDBConnectionString())
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse valkey url")
	}

	valkeyClient, err := valkey.NewClient(client)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to connect to valkey database")
	}
	if err := valkeyClient.Do(context.Background(), valkeyClient.B().Ping().Build()).Error(); err != nil {
		log.Fatal().Err(err).Msg("unable to ping to valkey database")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := grpc.NewClient(config.AccountServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect")
	}
	defer conn.Close()

	c := account.NewAccountServiceClient(conn)

	var wg sync.WaitGroup

	wg.Add(1)
	go handler.StartGrpcServer(ctx, config.GrpcServerAddress, valkeyClient, c, &wg)

	wg.Add(1)
	go handler.StartRestServer(ctx, config.RestServerAddress, valkeyClient, c, &wg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Info().Msg("received shutdown signal")
	cancel()

	wg.Wait()
}

package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/lmnzx/slopify/auth/handler"
	"github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/logger"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log := logger.Get()

	valkeyClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{"127.0.0.1:6379"},
		SelectDB:    1,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("unable to connect to valkey database")
	}
	if err := valkeyClient.Do(context.Background(), valkeyClient.B().Ping().Build()).Error(); err != nil {
		log.Fatal().Err(err).Msg("unable to ping to valkey database")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go startGrpcServer(ctx, valkeyClient, &log, &wg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Info().Msg("received shutdown signal")
	cancel()

	wg.Wait()
}

func startGrpcServer(ctx context.Context, valkeyClient valkey.Client, log *zerolog.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	lis, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup tcp listener")
		return
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(logger.UnaryServerInterceptor()),
	)

	h := handler.NewGrpcHandler(valkeyClient, log)
	proto.RegisterAuthServiceServer(s, h)
	reflection.Register(s)

	serveErrCh := make(chan error, 1)
	go func() {
		log.Info().Msg("grpc server stared")
		if err := s.Serve(lis); err != nil {
			if err != grpc.ErrServerStopped {
				serveErrCh <- err
			} else {
				close(serveErrCh)
			}
		} else {
			close(serveErrCh)
		}
	}()

	select {
	case <-ctx.Done():
		s.GracefulStop()
		if err := <-serveErrCh; err != nil {
			log.Error().Err(err).Msg("error during server run after shutdown initiated")
		}
	case err := <-serveErrCh:
		log.Error().Err(err).Msg("grpc server failed")
	}
}

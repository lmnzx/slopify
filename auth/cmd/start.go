package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/auth/handler"
	"github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/logger"

	"github.com/fasthttp/router"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	// account service
	conn, err := grpc.NewClient(":3000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect")
	}
	defer conn.Close()

	c := account.NewAccountServiceClient(conn)

	var wg sync.WaitGroup

	wg.Add(1)
	go startGrpcServer(ctx, valkeyClient, c, &log, &wg)

	wg.Add(1)
	go startRestServer(ctx, valkeyClient, c, &log, &wg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Info().Msg("received shutdown signal")
	cancel()

	wg.Wait()
}

func startGrpcServer(ctx context.Context, valkeyClient valkey.Client, accountService account.AccountServiceClient, log *zerolog.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	lis, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup tcp listener")
		return
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(logger.UnaryServerInterceptor()),
	)

	h := handler.NewGrpcHandler(valkeyClient, log, accountService)
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

func startRestServer(ctx context.Context, valkeyClient valkey.Client, accountService account.AccountServiceClient, log *zerolog.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	h := handler.NewRestHandler(valkeyClient, log, accountService)
	r := router.New()
	r.GET("/health", h.HealthCheck)
	r.POST("/signup", h.SignUp)
	r.POST("/login", h.LogIn)
	r.GET("/validate", h.ValidateSession)
	r.POST("/refresh", h.RefreshTokens)
	r.GET("/logout", h.LogOut)

	server := &fasthttp.Server{
		Handler: logger.RequestLogger(r.Handler),
	}

	serveErrCh := make(chan error, 1)
	go func() {
		restAddr := ":9001"
		log.Info().Msg("rest server started")
		if err := server.ListenAndServe(restAddr); err != nil {
			select {
			case <-ctx.Done():
				log.Println("fasthttp server stopped gracefully")
			default:
				serveErrCh <- err
			}
			close(serveErrCh)
		} else {
			close(serveErrCh)
		}
	}()

	select {
	case <-ctx.Done():
		if err := server.Shutdown(); err != nil {
			log.Error().Err(err).Msg("error during fasthttp server shutdown")
		}
		<-serveErrCh

	case err := <-serveErrCh:
		if err != nil {
			log.Error().Err(err).Msg("fasthttp server failed unexpectedly")
		}
	}
}

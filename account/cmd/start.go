package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/lmnzx/slopify/account/handler"
	"github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/account/repository"
	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/middleware"

	"github.com/fasthttp/router"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
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
	go startGrpcServer(ctx, queries, &wg)

	wg.Add(1)
	go startRestServer(ctx, queries, c, &wg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Info().Msg("received shutdown signal")
	cancel()

	wg.Wait()
}

func startGrpcServer(ctx context.Context, queries *repository.Queries, wg *sync.WaitGroup) {
	defer wg.Done()

	log := middleware.GetLogger()

	lis, err := net.Listen("tcp", ":3000")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup tcp listener")
		return
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor()),
	)

	h := handler.NewGrpcHandler(queries)
	proto.RegisterAccountServiceServer(s, h)
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

func startRestServer(ctx context.Context, queries *repository.Queries, auth auth.AuthServiceClient, wg *sync.WaitGroup) {
	defer wg.Done()

	r := router.New()

	handler := handler.NewRestHandler(queries, auth)
	authMw := middleware.AuthMiddleware(auth)

	r.GET("/health", handler.HealthCheck)
	r.POST("/update", authMw(handler.Update))

	server := &fasthttp.Server{
		Handler: middleware.RequestLogger(r.Handler),
	}

	log := middleware.GetLogger()
	serveErrCh := make(chan error, 1)
	go func() {
		restAddr := ":9000"
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

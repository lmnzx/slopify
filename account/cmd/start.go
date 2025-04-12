package main

import (
	"context"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmnzx/slopify/account/internal/handler"
	"github.com/lmnzx/slopify/pkg/logger"
	"github.com/lmnzx/slopify/pkg/validate"
	"github.com/valkey-io/valkey-go"
	"github.com/valyala/fasthttp"
)

func main() {
	l := logger.Get()
	ctx := context.Background()
	conn, err := pgxpool.New(ctx, "postgres://postgres:password@my-postgres.orb.local:5432/slopify?sslmode=disable")
	if err != nil {
		l.Error().Err(err).Msg("failed to create postgres connection pool")
		os.Exit(1)
	}
	defer conn.Close()

	connection, err := conn.Acquire(ctx)
	if err != nil {
		l.Error().Err(err).Msg("failed to acquire postgres connection")
		os.Exit(1)
	}
	defer connection.Release()
	if err := connection.Ping(ctx); err != nil {
		l.Error().Err(err).Msg("failed to ping postgres")
		os.Exit(1)
	}

	kv, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{"my-valkey.orb.local:6379"},
	})
	if err != nil {
		l.Error().Err(err).Msg("failed to create valkey client")
		os.Exit(1)
	}
	if err := kv.Do(ctx, kv.B().Ping().Build()).Error(); err != nil {
		l.Error().Err(err).Msg("failed to pind valkey")
		os.Exit(1)
	}
	defer kv.Close()

	handler := handler.New(conn, kv, &l)
	validatorMw := validate.ValidatorMiddleware(&validate.CustomValidator{Validator: validator.New()})

	l.Info().Msg("starting account-service on port 8080...")
	if err := fasthttp.ListenAndServe(":8080", logger.RequestLogger(validatorMw(handler.Router().Handler))); err != nil {
		l.Error().Err(err).Msg("failed to start account-service server")
		os.Exit(1)
	}
}

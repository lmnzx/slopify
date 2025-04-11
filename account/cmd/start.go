package main

import (
	"context"
	"os"

	"github.com/lmnzx/slopify/account/internal/handler"
	"github.com/lmnzx/slopify/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"
)

func main() {
	ctx := context.Background()
	conn, err := pgxpool.New(ctx, "postgres://postgres:password@localhost:5432/slopify?sslmode=disable")
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()

	handler := handler.NewHandler(conn)

	if err := fasthttp.ListenAndServe(":8080", logger.RequestLogger(handler.Router().Handler)); err != nil {
		os.Exit(1)
	}
}

package main

import (
	"os"

	"github.com/lmnzx/slopify/pkg/logger"
	"github.com/lmnzx/slopify/product/internal/handler"
	"github.com/valyala/fasthttp"
)

func main() {
	l := logger.Get()

	handler := handler.New()

	l.Info().Msg("starting product-service on port 8080...")
	if err := fasthttp.ListenAndServe(":8080", handler.Handler()); err != nil {
		l.Error().Err(err).Msg("failed to start product-service server")
		os.Exit(1)
	}
}

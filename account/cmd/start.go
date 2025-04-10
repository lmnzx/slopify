package main

import (
	"github.com/lmnzx/slopify/pkg/logger"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

func health_check(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Response.SetBodyString(`{"message": "all ok boss 👍🏻"}`)
}

func main() {
	r := router.New()

	r.GET("/health_check", logger.RequestLogger(health_check))

	l := logger.Get()

	if err := fasthttp.ListenAndServe(":8080", r.Handler); err != nil {
		l.Fatal().Err(err)
	}
}

package handler

import (
	"github.com/fasthttp/router"
	"github.com/go-playground/validator/v10"
	"github.com/lmnzx/slopify/pkg/logger"
	"github.com/lmnzx/slopify/pkg/validate"
	"github.com/valyala/fasthttp"
)

type handler struct{}

func New() *handler {
	return &handler{}
}

func (h *handler) Handler() fasthttp.RequestHandler {
	r := router.New()
	g := r.Group("/api/product")

	g.GET("/health_check", h.healthCheck)

	validatorMw := validate.ValidatorMiddleware(&validate.CustomValidator{Validator: validator.New()})
	t := logger.RequestLogger(validatorMw(r.Handler))

	return t
}

func (h *handler) healthCheck(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("application/json")
	ctx.Response.SetBodyString(`{"message": "all ok boss 👍🏻"}`)
}

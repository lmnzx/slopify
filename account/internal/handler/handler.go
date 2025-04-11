package handler

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fasthttp/router"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmnzx/slopify/account/internal/service"
	"github.com/valyala/fasthttp"
)

type handler struct {
	svc service.Service
}

func NewHandler(conn *pgxpool.Pool) *handler {
	return &handler{
		svc: service.NewService(conn),
	}
}

func (h *handler) Router() *router.Router {
	r := router.New()

	r.GET("/health_check", h.healthCheck)
	r.POST("/signup", h.signUp)
	r.POST("/login", h.logIn)

	return r
}

func (h *handler) healthCheck(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("application/json")
	ctx.Response.SetBodyString(`{"message": "all ok boss 👍🏻"}`)
}

func (h *handler) signUp(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	ctx.SetContentType("application/json")
	if len(body) == 0 {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.Response.SetBodyString(`{"error": "request body is required"}`)
		return
	}
	var parsedBody service.SignupRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.Response.SetBodyString(`{"error": "invalid JSON data"}`)
		return
	}

	if err := parsedBody.Validate(); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.Response.SetBodyString(fmt.Sprintf(`{"error": "%s"}`, err.Error()))
		return
	}

	var e *pgconn.PgError
	msg, err := h.svc.SignUp(ctx, &parsedBody)
	if errors.As(err, &e) && e.Code == pgerrcode.UniqueViolation {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.Response.SetBodyString(`{"error": "user already exists"}`)
		return
	}
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.Response.SetBodyString(`{"error": "failed to create user"}`)
		return
	}
	ctx.Response.SetBodyString(fmt.Sprintf(`{"message": "%s"}`, msg))
}

func (h *handler) logIn(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	ctx.SetContentType("application/json")
	if len(body) == 0 {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.Response.SetBodyString(`{"error": "request body is required"}`)
		return
	}
	var parsedBody service.LoginRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.Response.SetBodyString(`{"error": "invalid JSON data"}`)
		return
	}

	if err := parsedBody.Validate(); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.Response.SetBodyString(fmt.Sprintf(`{"error": "%s"}`, err.Error()))
		return
	}

	msg, err := h.svc.Login(ctx, &parsedBody)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.Response.SetBodyString(`{"error": "failed to authenticated user"}`)
		return
	}
	ctx.Response.SetBodyString(fmt.Sprintf(`{"message": "%s"}`, msg))
}

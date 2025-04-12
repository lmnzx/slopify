package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fasthttp/router"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmnzx/slopify/account/internal/service"
	"github.com/lmnzx/slopify/pkg/validate"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"github.com/valyala/fasthttp"
)

type handler struct {
	svc service.Service
}

func New(conn *pgxpool.Pool, kv valkey.Client, l *zerolog.Logger) *handler {
	return &handler{
		svc: service.NewService(conn, kv, l),
	}
}

func (h *handler) Router() *router.Router {
	r := router.New()
	g := r.Group("/api/account")

	g.GET("/health_check", h.healthCheck)
	g.POST("/signup", h.signUp)
	g.POST("/login", h.logIn)

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

	v := ctx.UserValue(validate.ValidatorKey).(*validate.CustomValidator)
	if err := v.Validate(parsedBody); err != nil {
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
	// TODO:refactor out
	cookie := fasthttp.AcquireCookie()
	cookie.SetKey("access_token")
	cookie.SetValue(msg.AccessToken)
	cookie.SetHTTPOnly(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	cookie.SetPath("/")
	cookie.SetExpire(time.Now().Add(service.AccessTokenExpiry))
	ctx.Request.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	ctx.Response.Header.SetCookie(cookie)
	fasthttp.ReleaseCookie(cookie)

	cookie = fasthttp.AcquireCookie()
	cookie.SetKey("refresh_token")
	cookie.SetValue(msg.RefreshToken)
	cookie.SetHTTPOnly(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	// TODO: route
	cookie.SetPath("/api/account/verify")
	cookie.SetExpire(time.Now().Add(service.RefreshTokenExpiry))
	ctx.Request.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	ctx.Response.Header.SetCookie(cookie)
	fasthttp.ReleaseCookie(cookie)

	ctx.Response.SetBodyString(`{"message": "welcome new user"}`)
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

	v := ctx.UserValue(validate.ValidatorKey).(*validate.CustomValidator)
	if err := v.Validate(parsedBody); err != nil {
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

	// TODO:refactor out
	cookie := fasthttp.AcquireCookie()
	cookie.SetKey("access_token")
	cookie.SetValue(msg.AccessToken)
	cookie.SetHTTPOnly(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	cookie.SetPath("/")
	cookie.SetExpire(time.Now().Add(service.AccessTokenExpiry))
	ctx.Request.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	ctx.Response.Header.SetCookie(cookie)
	fasthttp.ReleaseCookie(cookie)

	cookie = fasthttp.AcquireCookie()
	cookie.SetKey("refresh_token")
	cookie.SetValue(msg.RefreshToken)
	cookie.SetHTTPOnly(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	// TODO: route
	cookie.SetPath("/api/account/verify")
	cookie.SetExpire(time.Now().Add(service.RefreshTokenExpiry))
	ctx.Request.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	ctx.Response.Header.SetCookie(cookie)
	fasthttp.ReleaseCookie(cookie)

	ctx.Response.SetBodyString(`{"message": "user logeged in"}`)
}

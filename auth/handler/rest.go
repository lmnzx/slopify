package handler

import (
	"encoding/json"
	"time"

	"github.com/lmnzx/slopify/account/proto"
	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/auth/client"
	"github.com/lmnzx/slopify/auth/internal"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"github.com/valyala/fasthttp"
)

type RestHandler struct {
	service        internal.AuthService
	accountService account.AccountServiceClient
	log            *zerolog.Logger
}

func NewRestHandler(client valkey.Client, l *zerolog.Logger, a account.AccountServiceClient) *RestHandler {
	return &RestHandler{
		service:        *internal.NewAuthService(client, l),
		accountService: a,
		log:            l,
	}
}

type SignUpRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Address  string `json:"address"`
}

func (h *RestHandler) SignUp(ctx *fasthttp.RequestCtx) {
	body := ctx.Request.Body()
	if len(body) == 0 {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	var parsedBody SignUpRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	user, err := client.GetUser(ctx, h.log, h.accountService, parsedBody.Email)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	if user.UserId != "" {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return
	}

	req := proto.CreateUserRequest{
		Name:     parsedBody.Name,
		Email:    parsedBody.Email,
		Password: parsedBody.Password,
		Address:  parsedBody.Address,
	}

	createdUser, err := client.CreateUser(ctx, h.log, h.accountService, &req)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	tokenPair, err := h.service.GenerateTokenPair(ctx, createdUser.UserId, createdUser.Email)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	// todo helper function refactor
	cookie := fasthttp.AcquireCookie()
	cookie.SetKey("access_token")
	cookie.SetValue(tokenPair.AccessToken)
	cookie.SetHTTPOnly(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	cookie.SetPath("/")
	cookie.SetExpire(time.Now().Add(internal.AccessTokenExpiry))
	ctx.Request.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	ctx.Response.Header.SetCookie(cookie)
	fasthttp.ReleaseCookie(cookie)

	cookie = fasthttp.AcquireCookie()
	cookie.SetKey("refresh_token")
	cookie.SetValue(tokenPair.RefreshToken)
	cookie.SetHTTPOnly(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	cookie.SetPath("/")
	cookie.SetExpire(time.Now().Add(internal.RefreshTokenExpiry))
	ctx.Request.Header.SetCookieBytesKV(cookie.Key(), cookie.Value())
	ctx.Response.Header.SetCookie(cookie)
	fasthttp.ReleaseCookie(cookie)

	ctx.Response.SetBodyString(`{"message": "welcome"}`)
}

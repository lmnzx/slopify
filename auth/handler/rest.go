package handler

import (
	"encoding/json"

	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/auth/client"
	"github.com/lmnzx/slopify/auth/internal"
	"github.com/lmnzx/slopify/pkg/cookie"
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

	req := account.CreateUserRequest{
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

	cookie.Set(ctx, "access_token", tokenPair.AccessToken, "/", "", internal.AccessTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
	cookie.Set(ctx, "refresh_token", tokenPair.RefreshToken, "/", "", internal.RefreshTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)

	ctx.Response.SetBodyString(`{"message": "welcome"}`)
}

type LogInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *RestHandler) LogIn(ctx *fasthttp.RequestCtx) {
	body := ctx.Request.Body()
	if len(body) == 0 {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	var parsedBody LogInRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	checkPasswordReq := &account.VaildEmailPasswordRequest{
		Email:    parsedBody.Email,
		Password: parsedBody.Password,
	}

	isValid := client.CheckPassword(ctx, h.log, h.accountService, checkPasswordReq)

	if !isValid {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return
	}

	user, err := client.GetUser(ctx, h.log, h.accountService, parsedBody.Email)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	if user.UserId == "" {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	tokenPair, err := h.service.GenerateTokenPair(ctx, user.UserId, user.Email)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	cookie.Set(ctx, "access_token", tokenPair.AccessToken, "/", "", internal.AccessTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
	cookie.Set(ctx, "refresh_token", tokenPair.RefreshToken, "/", "", internal.RefreshTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)

	ctx.Response.SetBodyString(`{"message": "welcome back"}`)
}

func (h *RestHandler) LogOut(ctx *fasthttp.RequestCtx) {
	cookie.Delete(ctx, "access_token")
	cookie.Delete(ctx, "refresh_token")

	ctx.Response.SetBodyString(`{"message": "good bye"}`)
}

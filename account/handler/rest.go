package handler

import (
	"encoding/json"
	"time"

	"github.com/lmnzx/slopify/account/client"
	"github.com/lmnzx/slopify/account/repository"
	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/cookie"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

type RestHandler struct {
	queries     *repository.Queries
	authService auth.AuthServiceClient
	log         *zerolog.Logger
}

func NewRestHandler(queries *repository.Queries, a auth.AuthServiceClient, l *zerolog.Logger) *RestHandler {
	return &RestHandler{
		queries:     queries,
		authService: a,
		log:         l,
	}
}

type UpdateRequest struct {
	Email   string `json:"email"`
	Address string `json:"address"`
}

func (h *RestHandler) Update(ctx *fasthttp.RequestCtx) {
	body := ctx.Request.Body()
	if len(body) == 0 {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	var parsedBody UpdateRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	// TODO: auth middleware
	access_token := cookie.Get(ctx, "access_token")
	user_id := client.ValidateAccessToken(ctx, *h.log, h.authService, access_token)
	if user_id == "" {
		h.log.Info().Msg("no user was found with access_token trying refresh_token")

		refresh_token := cookie.Get(ctx, "refresh_token")
		tokenPair, err := client.ValidateRefreshToken(ctx, *h.log, h.authService, refresh_token)
		if err != nil {
			h.log.Error().Err(err).Msg("no user was found with refresh_token try login")
			ctx.Response.SetStatusCode(fasthttp.StatusForbidden)
			return
		}

		cookie.Set(ctx, "access_token", tokenPair.AccessToken, "/", "", time.Minute*15, false, fasthttp.CookieSameSiteDefaultMode)
		cookie.Set(ctx, "refresh_token", tokenPair.RefreshToken, "/", "", time.Hour*24*7, false, fasthttp.CookieSameSiteDefaultMode)

	}

	// changes the email token need to update
	updatedUser, err := h.queries.UpdateUser(ctx, repository.UpdateUserParams{Email: parsedBody.Email, Address: parsedBody.Address})
	if err != nil {
		h.log.Error().Err(err).Str("user_id", user_id).Msg("could not update the user")
		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	newTokenPair, err := client.GenerateTokenPair(ctx, *h.log, h.authService, updatedUser.ID.String(), updatedUser.Email)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", user_id).Msg("could generate token pair for the updated user")
		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	cookie.Set(ctx, "access_token", newTokenPair.AccessToken, "/", "", time.Minute*15, false, fasthttp.CookieSameSiteDefaultMode)
	cookie.Set(ctx, "refresh_token", newTokenPair.RefreshToken, "/", "", time.Hour*24*7, false, fasthttp.CookieSameSiteDefaultMode)

	ctx.Response.SetBodyString(`{"message": "user updated"}`)
}

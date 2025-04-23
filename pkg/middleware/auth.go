package middleware

import (
	"time"

	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/cookie"
	"github.com/valyala/fasthttp"
)

type ctxKey string

const UserIDCtxKey ctxKey = "userID"

func AuthMiddleware(authService auth.AuthServiceClient) func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			log := GetLogger()

			log.Info().Msg("auth middleware executing")

			accessToken := cookie.Get(ctx, "access_token")
			refreshToken := cookie.Get(ctx, "refresh_token")

			if accessToken == "" && refreshToken == "" {
				log.Warn().Msg("authMiddleware: access or refresh token missing from cookies")
				ctx.SetUserValue(UserIDCtxKey, "")
				next(ctx)
				return
			}

			r, err := authService.ValidateSession(ctx, &auth.TokenPair{
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			})
			if err != nil || r.Status != auth.ValidateSessionResponse_VALID || r == nil {
				log.Warn().Msg("authMiddleware: session validation failed")
				ctx.SetUserValue(UserIDCtxKey, "")
				next(ctx)
				return
			}

			log.Info().Str("userID", *r.UserId).Msg("authMiddleware: session validated successfully")

			cookie.Set(ctx, "access_token", r.TokenPair.AccessToken, "/", "", time.Minute*15, false, fasthttp.CookieSameSiteDefaultMode)
			if refreshToken != r.TokenPair.RefreshToken {
				cookie.Set(ctx, "refresh_token", r.TokenPair.RefreshToken, "/", "", time.Hour*24*7, false, fasthttp.CookieSameSiteDefaultMode)
			}

			ctx.SetUserValue(UserIDCtxKey, *r.UserId)

			next(ctx)
		}
	}
}

func GetUserIDFromCtx(ctx *fasthttp.RequestCtx) string {
	userIDVal := ctx.UserValue(UserIDCtxKey)
	if userIDVal == nil {
		return ""
	}

	userID, ok := userIDVal.(string)
	if !ok {
		return ""
	}
	return userID
}

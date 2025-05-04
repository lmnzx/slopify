package middleware

import (
	"context"
	"time"

	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/cookie"

	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const UserIDCtxKey string = "user_id"
const TracingCtxKey string = "tracing_context"

func AuthMiddleware(authService auth.AuthServiceClient, serviceName string) func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	tracer := otel.Tracer(serviceName)

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			log := GetLogger()
			log.Info().Msg("auth middleware executing")

			var spanCtx context.Context
			parentCtxVal := ctx.UserValue(TracingCtxKey)
			if parentCtxVal != nil {
				if parentContext, ok := parentCtxVal.(context.Context); ok {
					spanCtx = parentContext
				} else {
					spanCtx = context.Background()
				}
			} else {
				spanCtx = context.Background()
			}

			_, span := tracer.Start(
				spanCtx,
				"AuthMiddleware",
				trace.WithAttributes(
					attribute.String("middleware", "auth"),
					attribute.String("http.path", string(ctx.Path())),
					attribute.String("http.method", string(ctx.Method())),
				),
			)
			defer span.End()

			accessToken := cookie.Get(ctx, "access_token")
			refreshToken := cookie.Get(ctx, "refresh_token")

			if accessToken == "" && refreshToken == "" {
				log.Warn().Msg("authMiddleware: access or refresh token missing from cookies")
				span.SetAttributes(attribute.Bool("auth.success", false))
				ctx.SetUserValue(UserIDCtxKey, "")
				next(ctx)
				return
			}

			_, authSpan := tracer.Start(
				trace.ContextWithSpan(spanCtx, span),
				"ValidateSession",
			)

			r, err := authService.ValidateSession(ctx, &auth.TokenPair{
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			})
			if err != nil || r.Status != auth.ValidateSessionResponse_VALID || r == nil {
				log.Warn().Msg("authMiddleware: session validation failed")
				authSpan.SetAttributes(attribute.String("auth.middleware", "session validation failed"))
				ctx.SetUserValue(UserIDCtxKey, "")
				next(ctx)
				return
			}

			cookie.Set(ctx, "access_token", r.TokenPair.AccessToken, "/", "", time.Minute*15, false, fasthttp.CookieSameSiteDefaultMode)
			if refreshToken != r.TokenPair.RefreshToken {
				cookie.Set(ctx, "refresh_token", r.TokenPair.RefreshToken, "/", "", time.Hour*24*7, false, fasthttp.CookieSameSiteDefaultMode)
			}

			ctx.SetUserValue(UserIDCtxKey, *r.UserId)
			authSpan.SetAttributes(attribute.String("auth.user_id", *r.UserId))
			authSpan.End()

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

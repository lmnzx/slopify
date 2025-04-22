package client

import (
	"context"
	"time"

	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/rs/zerolog"
)

func ValidateAccessToken(ctx context.Context, log zerolog.Logger, c auth.AuthServiceClient, access_token string) string {
	ctx, cancle := context.WithTimeout(ctx, time.Second*5)
	defer cancle()

	r, err := c.ValidateToken(ctx, &auth.ValidateTokenRequest{AccessToken: access_token})
	if err != nil {
		log.Error().Err(err).Msg("could not validate token")
		return ""
	}
	if r.Status == auth.ValidateTokenResponse_VALID {
		log.Info().Str("status", r.Status.String()).Str("userid", *r.UserId).Msg("validated token")
		return *r.UserId
	}
	return ""
}

func ValidateRefreshToken(ctx context.Context, log zerolog.Logger, c auth.AuthServiceClient, refresh_token string) (*auth.TokenPair, error) {
	ctx, cancle := context.WithTimeout(ctx, time.Second*5)
	defer cancle()

	r, err := c.RefreshToken(ctx, &auth.RefreshTokenRequest{RefreshToken: refresh_token})
	if err != nil {
		log.Error().Err(err).Msg("could not validate token")
		return nil, err
	}
	return r, nil
}

func GenerateTokenPair(ctx context.Context, log zerolog.Logger, c auth.AuthServiceClient, user_id, email string) (*auth.TokenPair, error) {
	ctx, cancle := context.WithTimeout(ctx, time.Second*5)
	defer cancle()

	r, err := c.GenerateToken(ctx, &auth.GenerateTokenRequest{UserId: user_id, Email: email})
	if err != nil {
		log.Error().Err(err).Msg("could not validate token")
		return nil, err
	}
	return r, nil
}

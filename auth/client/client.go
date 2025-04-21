package client

import (
	"context"
	"time"

	"github.com/lmnzx/slopify/account/proto"
	"github.com/rs/zerolog"
)

func GetUser(ctx context.Context, log *zerolog.Logger, c proto.AccountServiceClient, email string) (*proto.User, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	r, err := c.GetUserByEmail(ctx, &proto.GetUserByEmailRequest{Email: email})
	if err != nil {
		log.Error().Err(err).Msg("could not get the user by email")
		return nil, err
	}

	log.Info().Str("user_id", r.UserId).Msg("fetched from auth service")
	return r, nil
}

func CreateUser(ctx context.Context, log *zerolog.Logger, c proto.AccountServiceClient, req *proto.CreateUserRequest) (*proto.User, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	r, err := c.CreateUser(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("could not create a user")
		return nil, err
	}

	log.Info().Str("user_id", r.UserId).Msg("created a new user")
	return r, nil
}

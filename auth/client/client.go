package client

import (
	"context"
	"time"

	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/pkg/middleware"
)

func GetUser(ctx context.Context, c account.AccountServiceClient, email string) (*account.User, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	log := middleware.GetLogger()

	r, err := c.GetUserByEmail(ctx, &account.GetUserByEmailRequest{Email: email})
	if err != nil {
		log.Error().Err(err).Msg("could not get the user by email")
		return nil, err
	}

	log.Info().Str("user_id", r.UserId).Msg("fetched from auth service")
	return r, nil
}

func CreateUser(ctx context.Context, c account.AccountServiceClient, req *account.CreateUserRequest) (*account.User, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	log := middleware.GetLogger()

	r, err := c.CreateUser(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("could not create a user")
		return nil, err
	}

	log.Info().Str("user_id", r.UserId).Msg("created a new user")
	return r, nil
}

func CheckPassword(ctx context.Context, c account.AccountServiceClient, req *account.VaildEmailPasswordRequest) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	log := middleware.GetLogger()

	r, err := c.VaildEmailPassword(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("could not create a user")
		return false
	}

	log.Info().Msg("valid login request")
	return r.IsValid
}

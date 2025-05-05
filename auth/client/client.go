package client

import (
	"context"
	"time"

	account "github.com/lmnzx/slopify/account/proto"
)

func GetUser(ctx context.Context, c account.AccountServiceClient, email string) (*account.User, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	r, err := c.GetUserByEmail(ctx, &account.GetUserByEmailRequest{Email: email})
	if err != nil {
		return nil, err
	}

	return r, nil
}

func CreateUser(ctx context.Context, c account.AccountServiceClient, req *account.CreateUserRequest) (*account.User, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	r, err := c.CreateUser(ctx, req)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func CheckPassword(ctx context.Context, c account.AccountServiceClient, req *account.VaildEmailPasswordRequest) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	r, err := c.VaildEmailPassword(ctx, req)
	if err != nil {
		return false
	}

	return r.IsValid
}

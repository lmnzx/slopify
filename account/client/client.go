package client

import (
	"context"
	"fmt"
	"time"

	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/rs/zerolog"
)

func ValidateSession(ctx context.Context, log *zerolog.Logger, c auth.AuthServiceClient, tokenPair *auth.TokenPair) (string, *auth.TokenPair) {
	ctx, cancle := context.WithTimeout(ctx, time.Second*5)
	defer cancle()

	r, err := c.ValidateSession(ctx, tokenPair)

	if err == nil && r.Status == auth.ValidateSessionResponse_VALID {
		fmt.Println(r)
		return *r.UserId, r.TokenPair
	}

	log.Warn().Msg("user session was not valid")
	return "", nil
}

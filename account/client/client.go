package client

import (
	"context"
	"time"

	"github.com/lmnzx/slopify/auth/proto"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func ValidateToken(ctx context.Context, log zerolog.Logger) {
	// auth server
	conn, err := grpc.NewClient(":8000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect")
	}
	defer conn.Close()

	c := proto.NewAuthServiceClient(conn)

	ctx, cancle := context.WithTimeout(ctx, time.Second*5)
	defer cancle()

	// test token
	r, err := c.ValidateToken(ctx, &proto.ValidateTokenRequest{AccessToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJVc2VySUQiOiIwMTk2NGZiNS1hMDRmLTcyODQtOTAzMi03MjEyODA1MTg3MmUgIiwiRW1haWwiOiJsQG0uY29tIiwiVHlwZSI6ImFjY2VzcyIsInN1YiI6IjAxOTY0ZmI1LWEwNGYtNzI4NC05MDMyLTcyMTI4MDUxODcyZSAiLCJleHAiOjE3NDUyMjQ2NTEsIm5iZiI6MTc0NTIyMzc1MSwiaWF0IjoxNzQ1MjIzNzUxLCJqdGkiOiI5ZTc3ZDZjYy0xN2Y3LTQxOTctYTQ2OC0yZDc0YjlmOGQ0OTAifQ.1V3RJVIujeDoSyID_o-DZ4vhkvG6vX-g183n1zRARJs"})
	if err != nil {
		log.Error().Err(err).Msg("could not validate token")
	}
	log.Info().Str("status", r.Status.String()).Str("userid", *r.UserId).Msg("validated token")
}

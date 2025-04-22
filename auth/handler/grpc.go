package handler

import (
	"context"

	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/auth/client"
	"github.com/lmnzx/slopify/auth/internal"
	"github.com/lmnzx/slopify/auth/proto"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcHandler struct {
	proto.UnimplementedAuthServiceServer
	service        internal.AuthService
	accountService account.AccountServiceClient
	log            *zerolog.Logger
}

func NewGrpcHandler(client valkey.Client, l *zerolog.Logger, a account.AccountServiceClient) *GrpcHandler {
	return &GrpcHandler{
		service:        *internal.NewAuthService(client, l),
		accountService: a,
		log:            l,
	}
}

func (h *GrpcHandler) GenerateToken(ctx context.Context, req *proto.GenerateTokenRequest) (*proto.TokenPair, error) {
	if req.Email == "" || req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid arguments")
	}

	user, err := client.GetUser(ctx, h.log, h.accountService, req.Email)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get the user from account service: %v", err)
	}

	if user.Email != req.Email || user.UserId != req.UserId {
		return nil, status.Errorf(codes.PermissionDenied, "user not found: %v", err)
	}

	tokenPair, err := h.service.GenerateTokenPair(ctx, req.UserId, req.Email)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "token pair generation failed: %v", err)
	}

	return &proto.TokenPair{AccessToken: tokenPair.AccessToken, RefreshToken: tokenPair.RefreshToken}, nil
}

func (h *GrpcHandler) ValidateToken(ctx context.Context, req *proto.ValidateTokenRequest) (*proto.ValidateTokenResponse, error) {
	if req.AccessToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid arguments")
	}

	userId, err := h.service.ValidateAccessToken(req.AccessToken)
	if err != nil {
		if err == internal.ErrTokenExpired {
			return &proto.ValidateTokenResponse{
				Status: proto.ValidateTokenResponse_EXPIRED,
				UserId: nil,
			}, nil
		}
		return &proto.ValidateTokenResponse{
			Status: proto.ValidateTokenResponse_INVALID,
			UserId: nil,
		}, nil

	}
	return &proto.ValidateTokenResponse{
		Status: proto.ValidateTokenResponse_VALID,
		UserId: &userId,
	}, nil
}

func (h *GrpcHandler) RefreshToken(ctx context.Context, req *proto.RefreshTokenRequest) (*proto.TokenPair, error) {
	if req.RefreshToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid arguments")
	}

	// TODO: don't generate everytime only gen new access token till Refresh Token is valid
	tokenPair, err := h.service.ValidateRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to validate refresh token: %v", err)
	}
	return &proto.TokenPair{AccessToken: tokenPair.AccessToken, RefreshToken: tokenPair.RefreshToken}, nil
}

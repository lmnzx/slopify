package handler

import (
	"context"

	"github.com/lmnzx/slopify/auth/internal"
	"github.com/lmnzx/slopify/auth/proto"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcHandler struct {
	proto.UnimplementedAuthServiceServer
	service internal.AuthService
}

func NewGrpcHandler(client valkey.Client, l *zerolog.Logger) *GrpcHandler {
	return &GrpcHandler{
		service: *internal.NewAuthService(client, l),
	}
}

func (h *GrpcHandler) GenerateToken(ctx context.Context, req *proto.GenerateTokenRequest) (*proto.TokenPair, error) {
	if req.Email == "" || req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid arguments")
	}

	// TODO: check email and userid from db
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
		return nil, status.Errorf(codes.PermissionDenied, "invail token")
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

	tokenPair, err := h.service.ValidateRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to validate refresh token: %v", err)
	}
	return &proto.TokenPair{AccessToken: tokenPair.AccessToken, RefreshToken: tokenPair.RefreshToken}, nil
}

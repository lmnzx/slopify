package handler

import (
	"context"
	"net"
	"sync"

	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/auth/client"
	"github.com/lmnzx/slopify/auth/internal"
	"github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/lmnzx/slopify/pkg/tracing"

	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type GrpcHandler struct {
	proto.UnimplementedAuthServiceServer
	authService    internal.AuthService
	accountService account.AccountServiceClient
	log            zerolog.Logger
	tracer         trace.Tracer
}

func NewGrpcHandler(client valkey.Client, accountService account.AccountServiceClient, secrets internal.Secrets) *GrpcHandler {
	return &GrpcHandler{
		authService:    *internal.NewAuthService(client, secrets),
		accountService: accountService,
		log:            middleware.GetLogger(),
		tracer:         otel.Tracer("auth-grpc-service"),
	}
}

func StartGrpcServer(ctx context.Context, port string, valkeyClient valkey.Client, accountService account.AccountServiceClient, secrets internal.Secrets, wg *sync.WaitGroup) {
	defer wg.Done()

	log := middleware.GetLogger()

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup tcp listener")
		return
	}

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.UnaryServerLoggingInterceptor(),
			tracing.UnaryServerTracingInterceptor("auth"),
		),
	)

	h := NewGrpcHandler(valkeyClient, accountService, secrets)
	proto.RegisterAuthServiceServer(s, h)
	reflection.Register(s)

	serveErrCh := make(chan error, 1)
	go func() {
		log.Info().Str("port", port).Msg("grpc server stared")
		if err := s.Serve(lis); err != nil {
			if err != grpc.ErrServerStopped {
				serveErrCh <- err
			} else {
				close(serveErrCh)
			}
		} else {
			close(serveErrCh)
		}
	}()

	select {
	case <-ctx.Done():
		s.GracefulStop()
		if err := <-serveErrCh; err != nil {
			log.Error().Err(err).Msg("error during server run after shutdown initiated")
		}
	case err := <-serveErrCh:
		log.Error().Err(err).Msg("grpc server failed")
	}
}

func (h *GrpcHandler) ValidateSession(ctx context.Context, req *proto.TokenPair) (*proto.ValidateSessionResponse, error) {
	if req.AccessToken == "" {
		if req.RefreshToken == "" {
			return &proto.ValidateSessionResponse{Status: proto.ValidateSessionResponse_EXPIRED}, status.Errorf(codes.Unauthenticated, "no access token provided")
		}

		tokenPair, err := h.authService.ValidateRefreshToken(ctx, req.RefreshToken)
		if err != nil {
			return &proto.ValidateSessionResponse{Status: proto.ValidateSessionResponse_EXPIRED}, status.Errorf(codes.Unauthenticated, "session expired")
		}

		req.AccessToken = tokenPair.AccessToken
		if tokenPair.RefreshToken != req.RefreshToken {
			req.RefreshToken = tokenPair.RefreshToken
		}
	}

	userId, err := h.authService.ValidateAccessToken(req.AccessToken)
	if err != nil {
		if err == internal.ErrTokenExpired && req.RefreshToken != "" {
			tokenPair, err := h.authService.ValidateRefreshToken(ctx, req.RefreshToken)
			if err != nil {
				return &proto.ValidateSessionResponse{Status: proto.ValidateSessionResponse_EXPIRED}, status.Errorf(codes.Unauthenticated, "session expired")
			}

			req.AccessToken = tokenPair.AccessToken
			if tokenPair.RefreshToken != req.RefreshToken {
				req.RefreshToken = tokenPair.RefreshToken
			}
			userId, err = h.authService.ValidateAccessToken(req.AccessToken)
			if err != nil {
				return &proto.ValidateSessionResponse{Status: proto.ValidateSessionResponse_INVALID}, status.Errorf(codes.Unauthenticated, "invalid session data")
			}

		} else {
			return &proto.ValidateSessionResponse{Status: proto.ValidateSessionResponse_INVALID}, status.Errorf(codes.Unauthenticated, "invalid session data")
		}
	}

	return &proto.ValidateSessionResponse{
		Status:    proto.ValidateSessionResponse_VALID,
		UserId:    &userId,
		TokenPair: req,
	}, nil
}

func (h *GrpcHandler) GenerateToken(ctx context.Context, req *proto.GenerateTokenRequest) (*proto.TokenPair, error) {
	if req.Email == "" || req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid arguments")
	}

	user, err := client.GetUser(ctx, h.accountService, req.Email)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get the user from account service: %v", err)
	}

	if user.Email != req.Email || user.UserId != req.UserId {
		h.log.Error().Str("userId", user.UserId).Str("email", user.Email).Msg("GenerateToken called with unregistred user")
		return nil, status.Errorf(codes.PermissionDenied, "user not found: %v", err)
	}

	tokenPair, err := h.authService.GenerateTokenPair(ctx, req.UserId, req.Email)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "token pair generation failed: %v", err)
	}

	return &proto.TokenPair{AccessToken: tokenPair.AccessToken, RefreshToken: tokenPair.RefreshToken}, nil
}

func (h *GrpcHandler) RefreshToken(ctx context.Context, req *proto.RefreshTokenRequest) (*proto.TokenPair, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	tokenPair, err := h.authService.ValidateRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to validate refresh token")

		switch err {
		case internal.ErrTokenExpired:
			return nil, status.Error(codes.Unauthenticated, "token expired")
		case internal.ErrInvalidToken, internal.ErrInvalidTokenClaims:
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		case internal.ErrTokenNotFound, internal.ErrTokenMismatch:
			return nil, status.Error(codes.Unauthenticated, "token revoked or invalid")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &proto.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

func (h *GrpcHandler) RevokeTokens(ctx context.Context, req *proto.RevokeTokensRequest) (*proto.RevokeTokensResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user ID is required")
	}

	err := h.authService.RevokeTokens(ctx, req.UserId)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to revoke tokens")
		return nil, status.Error(codes.Internal, "failed to revoke tokens")
	}

	return &proto.RevokeTokensResponse{
		Success: true,
	}, nil
}

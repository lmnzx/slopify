package handler

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/account/repository"
	"github.com/lmnzx/slopify/pkg/middleware"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GrpcHandler struct {
	proto.UnimplementedAccountServiceServer
	queries *repository.Queries
}

func NewGrpcHandler(queries *repository.Queries) *GrpcHandler {
	return &GrpcHandler{
		queries: queries,
	}
}

func StartGrpcServer(ctx context.Context, port string, queries *repository.Queries, wg *sync.WaitGroup) {
	defer wg.Done()

	log := middleware.GetLogger()

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup tcp listener")
		return
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor()),
	)

	h := NewGrpcHandler(queries)
	proto.RegisterAccountServiceServer(s, h)
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

func (h *GrpcHandler) GetUserById(ctx context.Context, req *proto.GetUserByIdRequest) (*proto.User, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	id, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user id: %v", err)
	}

	user, err := h.queries.GetUserById(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "no row") {
			return nil, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}
	return dbUserToProtoUser(&user), nil
}

func (h *GrpcHandler) GetUserByEmail(ctx context.Context, req *proto.GetUserByEmailRequest) (*proto.User, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	user, err := h.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if strings.Contains(err.Error(), "no row") {
			return nil, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	return dbUserToProtoUser(&user), nil
}

func (h *GrpcHandler) CreateUser(ctx context.Context, req *proto.CreateUserRequest) (*proto.User, error) {
	if req.Name == "" || req.Email == "" || req.Address == "" || req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing field")
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create uuid: %v", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to hash password: %v", err)
	}

	user, err := h.queries.CreateUser(ctx, repository.CreateUserParams{
		ID:       id,
		Name:     req.Name,
		Email:    req.Email,
		Address:  req.Address,
		Password: string(hashedPassword),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, status.Errorf(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	return dbUserToProtoUser(&user), nil
}

func (h *GrpcHandler) VaildEmailPassword(ctx context.Context, req *proto.VaildEmailPasswordRequest) (*proto.ValidResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing arguments")
	}

	user, err := h.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return &proto.ValidResponse{IsValid: false}, nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return &proto.ValidResponse{IsValid: false}, nil
	}
	return &proto.ValidResponse{IsValid: true}, nil
}

func dbUserToProtoUser(user *repository.User) *proto.User {
	return &proto.User{
		UserId:    user.ID.String(),
		Name:      user.Name,
		Email:     user.Email,
		Address:   user.Address,
		CreatedAt: timestamppb.New(user.CreatedAt),
		UpdatedAt: timestamppb.New(user.UpdatedAt),
	}
}

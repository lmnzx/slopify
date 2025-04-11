package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmnzx/slopify/account/internal/repository"
	"github.com/lmnzx/slopify/pkg/logger"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	SignUp(ctx context.Context, signupRequest *SignupRequest) (string, error)
	Login(ctx context.Context, loginRequest *LoginRequest) (string, error)
	// GetUser(ctx context.Context, email string) (*repository.User, error)
	// DeleteUser(ctx context.Context, email string) error
}

type accountService struct {
	pool *pgxpool.Pool
	db   *repository.Queries
	l    zerolog.Logger
}

func NewService(pool *pgxpool.Pool) Service {
	return accountService{
		pool: pool,
		db:   repository.New(pool),
		l:    logger.Get(),
	}
}

type SignupRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func (r *SignupRequest) Validate() error {
	if strings.TrimSpace(r.FirstName) == "" {
		return errors.New("first name is required")
	}
	if strings.TrimSpace(r.LastName) == "" {
		return errors.New("last name is required")
	}
	if strings.TrimSpace(r.Email) == "" {
		return errors.New("email is required")
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return errors.New("invalid email format")
	}
	if strings.TrimSpace(r.Password) == "" {
		return errors.New("password is required")
	}
	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	if strings.TrimSpace(r.Email) == "" {
		return errors.New("email is required")
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return errors.New("invalid email format")
	}
	if strings.TrimSpace(r.Password) == "" {
		return errors.New("password is required")
	}
	return nil
}

func (service accountService) SignUp(ctx context.Context, signupRequest *SignupRequest) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signupRequest.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := service.db.CreateUser(ctx, repository.CreateUserParams{
		FirstName: signupRequest.FirstName,
		LastName:  signupRequest.LastName,
		Email:     signupRequest.Email,
		Password:  string(hashedPassword),
	})
	if err != nil {
		service.l.Error().Err(err).Msg("failed to create new user")
		return "", fmt.Errorf("failed to create user: %w", err)
	}
	service.l.Info().Str("user_id", user.ID.String()).Str("email", user.Email).Msg("User created successfully")
	return fmt.Sprintf("created a new user with id: %s", user.ID), nil
}

func (service accountService) Login(ctx context.Context, loginRequest *LoginRequest) (string, error) {
	user, err := service.db.GetUser(ctx, loginRequest.Email)
	if err != nil {
		service.l.Error().Err(err).Msg("failed to get user")
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password))
	if err != nil {
		service.l.Info().Str("email", loginRequest.Email).Msg("failed login attempt")
		return "", fmt.Errorf("authentication failed")
	}

	service.l.Info().Str("user_id", user.ID.String()).Str("email", user.Email).Msg("user logged in")
	return fmt.Sprintf("user authenticated: %s", user.ID.String()), nil
}

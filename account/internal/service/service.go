package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmnzx/slopify/account/internal/repository"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"golang.org/x/crypto/bcrypt"
)

const (
	AccessTokenExpiry  = time.Minute * 15
	RefreshTokenExpiry = time.Hour * 24 * 7
	AccessTokenType    = "access"
	RefreshTokenType   = "refresh"
)

type Service interface {
	SignUp(ctx context.Context, signupRequest *SignupRequest) (*TokenPair, error)
	Login(ctx context.Context, loginRequest *LoginRequest) (*TokenPair, error)
	RefreshToken(ctx context.Context, refreshTokenString string) (*TokenPair, error)
	// GetUser(ctx context.Context, email string) (*repository.User, error)
	// DeleteUser(ctx context.Context, email string) error
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Type   string `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

type AuthConfig struct {
	AccessTokenSecret  string
	RefreshTokenSecret string
}

type accountService struct {
	db      *repository.Queries
	kv      valkey.Client
	l       *zerolog.Logger
	authCfg AuthConfig
}

func NewService(pool *pgxpool.Pool, kv valkey.Client, l *zerolog.Logger) Service {
	return &accountService{
		db: repository.New(pool),
		kv: kv,
		l:  l,
		authCfg: AuthConfig{
			AccessTokenSecret:  "testchangeinprod",
			RefreshTokenSecret: "testchangeinprod",
		},
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type SignupRequest struct {
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func (service *accountService) SignUp(ctx context.Context, signupRequest *SignupRequest) (*TokenPair, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signupRequest.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := service.db.CreateUser(ctx, repository.CreateUserParams{
		FirstName: signupRequest.FirstName,
		LastName:  signupRequest.LastName,
		Email:     signupRequest.Email,
		Password:  string(hashedPassword),
	})
	if err != nil {
		service.l.Error().Err(err).Msg("failed to create new user")
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	service.l.Info().Str("user_id", user.ID.String()).Str("email", user.Email).Msg("User created successfully")

	tokenPair, err := service.generateTokenPair(user.ID.String(), user.Email)
	if err != nil {
		service.l.Error().Err(err).Msg("failed to generate token pair")
		return nil, fmt.Errorf("failed to generate token pair: `%w`", err)
	}

	err = service.storeRefreshToken(ctx, user.ID.String(), tokenPair.RefreshToken)
	if err != nil {
		service.l.Error().Err(err).Msg("failed to store refresh token")
		return nil, fmt.Errorf("failed to store refresh token: `%w`", err)
	}
	return tokenPair, nil
}

func (service *accountService) Login(ctx context.Context, loginRequest *LoginRequest) (*TokenPair, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	user, err := service.db.GetUser(ctx, loginRequest.Email)
	if err != nil {
		service.l.Error().Err(err).Msg("failed to get user")
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password))
	if err != nil {
		service.l.Info().Str("email", loginRequest.Email).Msg("failed login attempt")
		return nil, fmt.Errorf("authentication failed")
	}

	service.l.Info().Str("user_id", user.ID.String()).Str("email", user.Email).Msg("user logged in")

	tokenPair, err := service.generateTokenPair(user.ID.String(), user.Email)
	if err != nil {
		service.l.Error().Err(err).Msg("failed to generate token pair")
		return nil, fmt.Errorf("failed to generate token pair: `%w`", err)
	}

	err = service.storeRefreshToken(ctx, user.ID.String(), tokenPair.RefreshToken)
	if err != nil {
		service.l.Error().Err(err).Msg("failed to store refresh token")
		return nil, fmt.Errorf("failed to store refresh token: `%w`", err)
	}
	return tokenPair, nil
}

// only for other services TODO
// func (service *accountService) GetUserEmail(ctx context.Context, accessTokenString string) (string, error) {
// 	token, err := jwt.ParseWithClaims(accessTokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
// 		return []byte(service.authCfg.AccessTokenSecret), nil
// 	})
// 	if err != nil {
// 		return "", fmt.Errorf("invalid refresh token: %w", err)
// 	}
//
// 	claims, ok := token.Claims.(*Claims)
// 	if !ok || token.Valid || claims.Type != AccessTokenType {
// 		return "", errors.New("invalid refresh token")
// 	}
//
// 	if time.Now().After(claims.ExpiresAt.Time)
//
// }

func (service *accountService) RefreshToken(ctx context.Context, refreshTokenString string) (*TokenPair, error) {
	token, err := jwt.ParseWithClaims(refreshTokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(service.authCfg.RefreshTokenSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || token.Valid || claims.Type != RefreshTokenType {
		return nil, errors.New("invalid refresh token")
	}

	ctx, cancle := context.WithTimeout(ctx, time.Second*5)
	defer cancle()

	storedToken, err := service.kv.Do(ctx, service.kv.B().Get().Key(claims.UserID).Build()).ToString()
	if err != nil || storedToken != refreshTokenString {
		return nil, errors.New("refresh token revoked or expired")
	}

	tokenPair, err := service.generateTokenPair(claims.UserID, claims.Email)
	if err != nil {
		service.l.Error().Err(err).Msg("failed to generate token pair")
		return nil, fmt.Errorf("failed to generate token pair: `%w`", err)
	}

	err = service.storeRefreshToken(ctx, claims.UserID, tokenPair.RefreshToken)
	if err != nil {
		service.l.Error().Err(err).Msg("failed to store refresh token")
		return nil, fmt.Errorf("failed to store refresh token: `%w`", err)
	}
	return tokenPair, nil
}

func (service *accountService) generateTokenPair(userID, email string) (*TokenPair, error) {
	accessTokenClaims := Claims{
		UserID: userID,
		Email:  email,
		Type:   AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	// TODO: RS256
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString([]byte(service.authCfg.AccessTokenSecret))
	if err != nil {
		service.l.Error().Err(err).Msg("failed to create access token")
		return nil, err
	}

	refreshTokenClaims := Claims{
		UserID: userID,
		Email:  email,
		Type:   RefreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(RefreshTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(service.authCfg.RefreshTokenSecret))
	if err != nil {
		service.l.Error().Err(err).Msg("failed to create refresh token")
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
	}, nil
}

// TODO: error handling
func (service *accountService) storeRefreshToken(ctx context.Context, userID, token string) error {
	service.kv.DoMulti(ctx, service.kv.B().Set().Key(userID).Value(token).Build(),
		service.kv.B().Expire().Key(userID).Seconds(int64(RefreshTokenExpiry.Seconds())).Build())
	return nil
}

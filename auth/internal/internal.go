package internal

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
)

const (
	AccessTokenExpiry  = time.Minute * 15
	RefreshTokenExpiry = time.Hour * 24 * 7
	AccessTokenType    = "access"
	RefreshTokenType   = "refresh"
)

type AuthConfig struct {
	AccessTokenSecret  string
	RefreshTokenSecret string
}

type Claims struct {
	UserID string
	Email  string
	Type   string
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type AuthService struct {
	kv      valkey.Client
	l       *zerolog.Logger
	authCfg AuthConfig
}

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidTokenClaims = errors.New("invalid token claims")
	ErrTokenExpired       = errors.New("token expired")
)

func NewAuthService(kv valkey.Client, l *zerolog.Logger) *AuthService {
	return &AuthService{
		kv: kv,
		l:  l,
		authCfg: AuthConfig{
			AccessTokenSecret:  "test",
			RefreshTokenSecret: "test",
		},
	}
}

func (s *AuthService) ValidateRefreshToken(ctx context.Context, token string) (*TokenPair, error) {
	refreshToken, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.authCfg.AccessTokenSecret), nil
	})
	if err != nil {
		s.l.Error().Err(err).Msg("invalid token")
		return nil, ErrInvalidToken
	}

	claims, ok := refreshToken.Claims.(*Claims)
	if !ok || !refreshToken.Valid || claims.Type != AccessTokenType {
		s.l.Error().Err(err).Msg("invalid token claims")
		return nil, ErrInvalidTokenClaims
	}

	storedToken, err := s.kv.Do(ctx, s.kv.B().Get().Key(claims.UserID).Build()).ToString()
	if err != nil || storedToken != token {
		return nil, ErrTokenExpired
	}

	tokenPair, err := s.GenerateTokenPair(ctx, claims.UserID, claims.Email)
	if err != nil {
		return nil, err
	}

	return tokenPair, nil
}

func (s *AuthService) ValidateAccessToken(token string) (string, error) {
	accessToken, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.authCfg.AccessTokenSecret), nil
	})
	if err != nil {
		s.l.Error().Err(err).Msg("invalid token")
		return "", ErrInvalidToken
	}

	claims, ok := accessToken.Claims.(*Claims)
	if !ok || !accessToken.Valid || claims.Type != AccessTokenType {
		s.l.Error().Err(err).Msg("invalid token claims")
		return "", ErrInvalidTokenClaims
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		s.l.Error().Err(err).Msg("token expire")
		return "", ErrTokenExpired
	}

	return claims.UserID, nil
}

func (s *AuthService) GenerateTokenPair(ctx context.Context, userID, email string) (*TokenPair, error) {
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
	accessTokenString, err := accessToken.SignedString([]byte(s.authCfg.AccessTokenSecret))
	if err != nil {
		s.l.Error().Err(err).Msg("failed to create access token")
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
	refreshTokenString, err := refreshToken.SignedString([]byte(s.authCfg.RefreshTokenSecret))
	if err != nil {
		s.l.Error().Err(err).Msg("failed to create refresh token")
		return nil, err
	}

	s.kv.DoMulti(ctx, s.kv.B().Set().Key(userID).Value(refreshTokenString).Build(),
		s.kv.B().Expire().Key(userID).Seconds(int64(RefreshTokenExpiry.Seconds())).Build())

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
	}, nil
}

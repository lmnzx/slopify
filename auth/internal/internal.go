package internal

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
)

const (
	AccessTokenExpiry          = time.Minute * 15
	RefreshTokenExpiry         = time.Hour * 24 * 7
	RefreshTokenReuseThreshold = time.Hour * 24
	AccessTokenType            = "access"
	RefreshTokenType           = "refresh"
)

type Secrets struct {
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
	log     zerolog.Logger
	secrets Secrets
}

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidTokenClaims = errors.New("invalid token claims")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenNotFound      = errors.New("token not found in storage")
	ErrTokenMismatch      = errors.New("token does not match stored token")
)

func NewAuthService(kv valkey.Client, secrets Secrets) *AuthService {
	return &AuthService{
		kv:  kv,
		log: middleware.GetLogger(),
		secrets: Secrets{
			AccessTokenSecret:  secrets.AccessTokenSecret,
			RefreshTokenSecret: secrets.RefreshTokenSecret,
		},
	}
}

func (s *AuthService) ValidateRefreshToken(ctx context.Context, token string) (*TokenPair, error) {
	refreshToken, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.secrets.AccessTokenSecret), nil
	})
	if err != nil {
		s.log.Error().Err(err).Msg("invalid token")
		return nil, ErrInvalidToken
	}

	claims, ok := refreshToken.Claims.(*Claims)
	if !ok || !refreshToken.Valid {
		s.log.Error().Msg("invalid token claims format")
		return nil, ErrInvalidTokenClaims
	}

	if claims.Type != RefreshTokenType {
		s.log.Error().Str("type", claims.Type).Msg("invalid token type")
		return nil, ErrInvalidTokenClaims
	}

	storedToken, err := s.kv.Do(ctx, s.kv.B().Get().Key(claims.UserID).Build()).ToString()
	if err != nil {
		s.log.Error().Err(err).Str("userId", claims.UserID).Msg("failed to get stored token")
		return nil, ErrTokenNotFound
	}

	if storedToken != token {
		s.log.Error().Str("userId", claims.UserID).Msg("token doesn't match stored token")
		return nil, ErrTokenMismatch
	}

	accessTokenString, err := s.generateAccessToken(claims.UserID, claims.Email)
	if err != nil {
		return nil, err
	}

	var refreshTokenString string
	timeUntilExpiry := time.Until(claims.ExpiresAt.Time)

	if timeUntilExpiry < RefreshTokenReuseThreshold {
		s.log.Info().Str("userId", claims.UserID).Dur("timeUntilExpiry", timeUntilExpiry).Msg("refresh token close to expiry, generating new one")

		newRefreshToken, err := s.generateRefreshToken(ctx, claims.UserID, claims.Email)
		if err != nil {
			return nil, err
		}
		refreshTokenString = newRefreshToken
	} else {
		s.log.Info().Str("userId", claims.UserID).Dur("timeUntilExpiry", timeUntilExpiry).Msg("reusing existing refresh token")
		refreshTokenString = storedToken
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
	}, nil
}

func (s *AuthService) ValidateAccessToken(token string) (string, error) {
	accessToken, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.secrets.AccessTokenSecret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrTokenExpired
		}
		s.log.Error().Err(err).Msg("invalid access token")
		return "", ErrInvalidToken
	}

	claims, ok := accessToken.Claims.(*Claims)
	if !ok || !accessToken.Valid || claims.Type != AccessTokenType {
		s.log.Error().Err(err).Msg("invalid token claims")
		return "", ErrInvalidTokenClaims
	}

	return claims.UserID, nil
}

func (s *AuthService) GenerateTokenPair(ctx context.Context, userID, email string) (*TokenPair, error) {
	accessTokenString, err := s.generateAccessToken(userID, email)
	if err != nil {
		return nil, err
	}

	refreshTokenString, err := s.generateRefreshToken(ctx, userID, email)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
	}, nil
}

func (s *AuthService) RevokeTokens(ctx context.Context, userID string) error {
	result := s.kv.Do(ctx, s.kv.B().Del().Key(userID).Build())
	if result.Error() != nil {
		s.log.Error().Err(result.Error()).Str("userId", userID).Msg("failed to revoke tokens")
		return result.Error()
	}
	return nil
}

func (s *AuthService) generateAccessToken(userID, email string) (string, error) {
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

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.secrets.AccessTokenSecret))
	if err != nil {
		s.log.Error().Err(err).Msg("failed to create access token")
		return "", err
	}

	return accessTokenString, nil
}

func (s *AuthService) generateRefreshToken(ctx context.Context, userID, email string) (string, error) {
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
	refreshTokenString, err := refreshToken.SignedString([]byte(s.secrets.RefreshTokenSecret))
	if err != nil {
		s.log.Error().Err(err).Msg("failed to create refresh token")
		return "", err
	}

	err = s.storeRefreshToken(ctx, userID, refreshTokenString)
	if err != nil {
		return "", err
	}

	return refreshTokenString, nil
}

// TODO: Error Handling
func (s *AuthService) storeRefreshToken(ctx context.Context, userID, token string) error {
	s.kv.DoMulti(ctx, s.kv.B().Set().Key(userID).Value(token).Build(),
		s.kv.B().Expire().Key(userID).Seconds(int64(RefreshTokenExpiry.Seconds())).Build())

	return nil
}

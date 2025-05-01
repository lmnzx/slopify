package handler

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/fasthttp/router"
	account "github.com/lmnzx/slopify/account/proto"
	"github.com/lmnzx/slopify/auth/client"
	"github.com/lmnzx/slopify/auth/internal"
	"github.com/lmnzx/slopify/pkg/cookie"
	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/lmnzx/slopify/pkg/response"
	"github.com/rs/zerolog"
	"github.com/valkey-io/valkey-go"
	"github.com/valyala/fasthttp"
)

type RestHandler struct {
	authService    internal.AuthService
	accountService account.AccountServiceClient
	res            *response.ResponseSender
	log            zerolog.Logger
}

func NewRestHandler(valkeyClient valkey.Client, accountService account.AccountServiceClient) *RestHandler {
	return &RestHandler{
		authService:    *internal.NewAuthService(valkeyClient),
		accountService: accountService,
		res:            response.NewResponseSender(),
		log:            middleware.GetLogger(),
	}
}

func StartRestServer(ctx context.Context, port string, valkeyClient valkey.Client, accountService account.AccountServiceClient, wg *sync.WaitGroup) {
	defer wg.Done()

	log := middleware.GetLogger()

	handler := NewRestHandler(valkeyClient, accountService)
	r := router.New()

	r.GET("/health", handler.HealthCheck)
	r.POST("/signup", handler.SignUp)
	r.POST("/login", handler.LogIn)
	r.GET("/validate", handler.ValidateSession)
	r.POST("/refresh", handler.RefreshTokens)
	r.GET("/logout", handler.LogOut)

	server := &fasthttp.Server{
		Handler: middleware.RequestLogger(r.Handler),
	}

	serveErrCh := make(chan error, 1)
	go func() {
		log.Info().Str("port", port).Msg("rest server started")
		if err := server.ListenAndServe(port); err != nil {
			select {
			case <-ctx.Done():
				log.Println("fasthttp server stopped gracefully")
			default:
				serveErrCh <- err
			}
			close(serveErrCh)
		} else {
			close(serveErrCh)
		}
	}()

	select {
	case <-ctx.Done():
		if err := server.Shutdown(); err != nil {
			log.Error().Err(err).Msg("error during fasthttp server shutdown")
		}
		<-serveErrCh

	case err := <-serveErrCh:
		if err != nil {
			log.Error().Err(err).Msg("fasthttp server failed unexpectedly")
		}
	}
}

func (h *RestHandler) HealthCheck(ctx *fasthttp.RequestCtx) {
	h.res.SendSuccess(ctx, fasthttp.StatusOK, "all ok")
}

type SignUpRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Address  string `json:"address"`
}

func (h *RestHandler) SignUp(ctx *fasthttp.RequestCtx) {
	body := ctx.Request.Body()
	if len(body) == 0 {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "request body cannot be empty")
		return
	}

	var parsedBody SignUpRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "invalid request format")
		return
	}

	if parsedBody.Email == "" || parsedBody.Password == "" || parsedBody.Name == "" {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "name, email, and password are required")
		return
	}

	user, err := client.GetUser(ctx, h.accountService, parsedBody.Email)
	if err != nil {
		h.log.Error().Err(err).Str("email", parsedBody.Email).Msg("error checking existing user")
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "error checking existing user")
		return
	}
	if user.UserId != "" {
		h.res.SendError(ctx, fasthttp.StatusConflict, "user already exists")
		return
	}

	req := account.CreateUserRequest{
		Name:     parsedBody.Name,
		Email:    parsedBody.Email,
		Password: parsedBody.Password,
		Address:  parsedBody.Address,
	}

	createdUser, err := client.CreateUser(ctx, h.accountService, &req)
	if err != nil {
		h.log.Error().Err(err).Str("email", parsedBody.Email).Msg("failed to create user")
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "failed to create user")
		return
	}

	tokenPair, err := h.authService.GenerateTokenPair(ctx, createdUser.UserId, createdUser.Email)
	if err != nil {
		h.log.Error().Err(err).Str("userId", createdUser.UserId).Msg("failed to generate tokens")
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "failed to generate authentication tokens")
		return
	}

	cookie.Set(ctx, "access_token", tokenPair.AccessToken, "/", "", internal.AccessTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
	cookie.Set(ctx, "refresh_token", tokenPair.RefreshToken, "/", "", internal.RefreshTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)

	h.res.SendSuccess(ctx, fasthttp.StatusCreated, map[string]string{
		"user_id": createdUser.UserId,
		"email":   createdUser.Email,
	})
}

type LogInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *RestHandler) LogIn(ctx *fasthttp.RequestCtx) {
	body := ctx.Request.Body()
	if len(body) == 0 {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "request body cannot be empty")
		return
	}

	var parsedBody LogInRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "invalid request format")
		return
	}

	if parsedBody.Email == "" || parsedBody.Password == "" {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "email and password are required")
		return
	}

	checkPasswordReq := &account.VaildEmailPasswordRequest{
		Email:    parsedBody.Email,
		Password: parsedBody.Password,
	}

	isValid := client.CheckPassword(ctx, h.accountService, checkPasswordReq)

	if !isValid {
		h.res.SendError(ctx, fasthttp.StatusUnauthorized, "invalid email or password")
		h.log.Error().Str("email", parsedBody.Email).Msg("user invalid credentials")
		return
	}

	user, err := client.GetUser(ctx, h.accountService, parsedBody.Email)
	if err != nil {
		h.log.Error().Err(err).Str("email", parsedBody.Email).Msg("failed to fetch user")
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "failed to fetch user details")
		return
	}
	if user == nil || user.UserId == "" {
		h.res.SendError(ctx, fasthttp.StatusNotFound, "user not found")
		return
	}

	tokenPair, err := h.authService.GenerateTokenPair(ctx, user.UserId, user.Email)
	if err != nil {
		h.log.Error().Err(err).Str("userId", user.UserId).Msg("failed to generate tokens")
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "failed to generate authentication tokens")
		return
	}

	cookie.Set(ctx, "access_token", tokenPair.AccessToken, "/", "", internal.AccessTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
	cookie.Set(ctx, "refresh_token", tokenPair.RefreshToken, "/", "", internal.RefreshTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)

	h.res.SendSuccess(ctx, fasthttp.StatusCreated, map[string]string{
		"user_id": user.UserId,
		"email":   user.Email,
	})
}

func (h *RestHandler) LogOut(ctx *fasthttp.RequestCtx) {
	accessToken := cookie.Get(ctx, "access_token")
	refreshToken := cookie.Get(ctx, "refresh_token")

	if accessToken == "" {
		if refreshToken == "" {
			h.res.SendError(ctx, fasthttp.StatusUnauthorized, "already logged out")
			return
		}
		tokenPair, err := h.authService.ValidateRefreshToken(ctx, refreshToken)
		if err != nil {
			h.res.SendError(ctx, fasthttp.StatusUnauthorized, "already logged out")
			return
		}
		accessToken = tokenPair.AccessToken
	}

	userId, err := h.authService.ValidateAccessToken(accessToken)
	if err != nil {
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "failed to logged out")
		return
	}

	err = h.authService.RevokeTokens(ctx, userId)
	if err != nil {
		h.log.Error().Err(err).Str("userId", userId).Msg("failed to revoke tokens during logout")
	}

	cookie.Delete(ctx, "access_token")
	cookie.Delete(ctx, "refresh_token")

	h.res.SendSuccess(ctx, fasthttp.StatusCreated, map[string]string{
		"message": "logged out successfully",
	})
}

func (h *RestHandler) RefreshTokens(ctx *fasthttp.RequestCtx) {
	refreshToken := cookie.Get(ctx, "refresh_token")
	if refreshToken == "" {
		h.res.SendError(ctx, fasthttp.StatusUnauthorized, "refresh token is required")
		return
	}

	tokenPair, err := h.authService.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to validate refresh token")
		h.res.SendError(ctx, fasthttp.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	cookie.Set(ctx, "access_token", tokenPair.AccessToken, "/", "", internal.AccessTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)

	// Only set refresh token if it's different (a new one was generated)
	if tokenPair.RefreshToken != refreshToken {
		cookie.Set(ctx, "refresh_token", tokenPair.RefreshToken, "/", "", internal.RefreshTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
	}

	h.res.SendSuccess(ctx, fasthttp.StatusCreated, map[string]string{
		"message": "tokens refreshed successfully",
	})
}

func (h *RestHandler) ValidateSession(ctx *fasthttp.RequestCtx) {
	accessToken := cookie.Get(ctx, "access_token")
	refreshToken := cookie.Get(ctx, "refresh_token")

	if accessToken == "" {
		if refreshToken == "" {
			h.res.SendError(ctx, fasthttp.StatusUnauthorized, "no access token provided")
			return
		}

		tokenPair, err := h.authService.ValidateRefreshToken(ctx, refreshToken)
		if err != nil {
			h.res.SendError(ctx, fasthttp.StatusUnauthorized, "session expired")
			return
		}

		cookie.Set(ctx, "access_token", tokenPair.AccessToken, "/", "", internal.AccessTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
		if tokenPair.RefreshToken != refreshToken {
			cookie.Set(ctx, "refresh_token", tokenPair.RefreshToken, "/", "", internal.RefreshTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
		}
		accessToken = tokenPair.AccessToken
	}

	userId, err := h.authService.ValidateAccessToken(accessToken)
	if err != nil {
		if err == internal.ErrTokenExpired && refreshToken != "" {
			tokenPair, err := h.authService.ValidateRefreshToken(ctx, refreshToken)
			if err != nil {
				h.res.SendError(ctx, fasthttp.StatusUnauthorized, "session expired")
				return
			}

			cookie.Set(ctx, "access_token", tokenPair.AccessToken, "/", "", internal.AccessTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
			if tokenPair.RefreshToken != refreshToken {
				cookie.Set(ctx, "refresh_token", tokenPair.RefreshToken, "/", "", internal.RefreshTokenExpiry, false, fasthttp.CookieSameSiteDefaultMode)
			}

			userId, err = h.authService.ValidateAccessToken(tokenPair.AccessToken)
			if err != nil {
				h.log.Error().Err(err).Msg("failed to validate new access token")
				h.res.SendError(ctx, fasthttp.StatusInternalServerError, "error validating session")
				return
			}
		} else {
			h.res.SendError(ctx, fasthttp.StatusUnauthorized, "invalid session")
			return
		}
	}

	h.res.SendSuccess(ctx, fasthttp.StatusCreated, map[string]string{
		"user_id": userId,
	})
}

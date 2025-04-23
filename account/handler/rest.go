package handler

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/fasthttp/router"
	"github.com/google/uuid"
	"github.com/lmnzx/slopify/account/repository"
	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/lmnzx/slopify/pkg/response"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

type RestHandler struct {
	queries *repository.Queries
	res     *response.ResponseSender
	log     zerolog.Logger
}

func NewRestHandler(queries *repository.Queries) *RestHandler {
	return &RestHandler{
		queries: queries,
		log:     middleware.GetLogger(),
		res:     response.NewResponseSender(),
	}
}

func StartRestServer(ctx context.Context, queries *repository.Queries, auth auth.AuthServiceClient, wg *sync.WaitGroup) {
	defer wg.Done()

	r := router.New()

	handler := NewRestHandler(queries)
	authMw := middleware.AuthMiddleware(auth)

	r.GET("/health", handler.healthCheck)
	r.POST("/update", authMw(handler.update))

	server := &fasthttp.Server{
		Handler: middleware.RequestLogger(r.Handler),
	}

	log := middleware.GetLogger()
	serveErrCh := make(chan error, 1)
	go func() {
		restAddr := ":9000"
		log.Info().Str("address", restAddr).Msg("rest server started")
		if err := server.ListenAndServe(restAddr); err != nil {
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

func (h *RestHandler) healthCheck(ctx *fasthttp.RequestCtx) {
	h.res.SendSuccess(ctx, fasthttp.StatusOK, "all ok")
}

type UpdateRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func (h *RestHandler) update(ctx *fasthttp.RequestCtx) {
	user_id := middleware.GetUserIDFromCtx(ctx)

	if user_id == "" {
		h.log.Info().Msg("attempt to update profile while not logged in")
		h.res.SendError(ctx, fasthttp.StatusUnauthorized, "user is not logged in")
		return
	}

	body := ctx.Request.Body()
	if len(body) == 0 {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "empty request body")
		return
	}

	var parsedBody UpdateRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "invalid request format, needs name and address to update")
		return
	}

	if parsedBody.Name == "" && parsedBody.Address == "" {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "no fields to update")
		return
	}

	id, err := uuid.Parse(user_id)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", user_id).Msg("could not parse the user_id")
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "could not parse the user_id")
		return
	}

	updatedUser, err := h.queries.UpdateUser(ctx, repository.UpdateUserParams{ID: id, Name: parsedBody.Name, Address: parsedBody.Address})
	if err != nil {
		h.log.Error().Err(err).Str("user_id", user_id).Msg("could not update the user")
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "could not update the user")
		return
	}

	h.res.SendSuccess(ctx, fasthttp.StatusOK, map[string]string{
		"user_id": updatedUser.ID.String(),
		"name":    updatedUser.Name,
		"address": updatedUser.Address,
	})
}

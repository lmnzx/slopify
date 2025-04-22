package handler

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/lmnzx/slopify/account/repository"
	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/lmnzx/slopify/pkg/response"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

type RestHandler struct {
	queries     *repository.Queries
	authService auth.AuthServiceClient
	res         *response.ResponseSender
	log         *zerolog.Logger
}

func NewRestHandler(queries *repository.Queries, authService auth.AuthServiceClient, log *zerolog.Logger) *RestHandler {
	return &RestHandler{
		queries:     queries,
		authService: authService,
		log:         log,
		res:         response.NewResponseSender(log),
	}
}

type UpdateRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func (h *RestHandler) Update(ctx *fasthttp.RequestCtx) {
	user_id := middleware.GetUserIDFromCtx(ctx)

	body := ctx.Request.Body()
	if len(body) == 0 {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	var parsedBody UpdateRequest
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
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
		"name":    updatedUser.Email,
		"address": updatedUser.Address,
	})
}

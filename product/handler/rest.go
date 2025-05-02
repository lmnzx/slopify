package handler

import (
	"context"
	"sync"

	"github.com/fasthttp/router"
	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/lmnzx/slopify/pkg/response"
	"github.com/lmnzx/slopify/product/repository"
	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

type RestHandler struct {
	index   meilisearch.IndexManager
	queries *repository.Queries
	res     *response.ResponseSender
	log     zerolog.Logger
}

func NewRestHandler(queries *repository.Queries, index meilisearch.IndexManager) *RestHandler {
	return &RestHandler{
		index:   index,
		queries: queries,
		log:     middleware.GetLogger(),
		res:     response.NewResponseSender(),
	}
}

func StartRestServer(ctx context.Context, port string, queries *repository.Queries, index meilisearch.IndexManager, auth auth.AuthServiceClient, wg *sync.WaitGroup) {
	defer wg.Done()

	r := router.New()

	handler := NewRestHandler(queries, index)
	authMw := middleware.AuthMiddleware(auth)

	r.GET("/health", handler.healthCheck)
	r.GET("/get", authMw(handler.getProduct))

	server := &fasthttp.Server{
		Handler: middleware.RequestID(middleware.RequestLogger(r.Handler)),
	}

	log := middleware.GetLogger()
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

func (h *RestHandler) healthCheck(ctx *fasthttp.RequestCtx) {
	h.res.SendSuccess(ctx, fasthttp.StatusOK, "all ok")
}

func (h *RestHandler) getProduct(ctx *fasthttp.RequestCtx) {
	user_id := middleware.GetUserIDFromCtx(ctx)

	query := ctx.QueryArgs().Peek("query")

	if len(query) == 0 {
		h.res.SendSuccess(ctx, fasthttp.StatusOK, "nothing to search")
	}

	if user_id == "" {
		h.log.Info().Msg("no user was found")
	}
	h.log.Info().Str("user_id", user_id).Msg("user found")

	searchRes, err := h.index.Search(string(query), &meilisearch.SearchRequest{
		Limit: 10,
	})
	if err != nil {
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "failed to get products from the search")
	}

	h.res.SendSuccess(ctx, fasthttp.StatusOK, searchRes.Hits)
}

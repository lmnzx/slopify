package handler

import (
	"context"
	"sync"

	auth "github.com/lmnzx/slopify/auth/proto"
	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/lmnzx/slopify/pkg/response"
	"github.com/lmnzx/slopify/pkg/tracing"
	"github.com/lmnzx/slopify/product/repository"

	"github.com/fasthttp/router"
	"github.com/meilisearch/meilisearch-go"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type RestHandler struct {
	index   meilisearch.IndexManager
	queries *repository.Queries
	res     *response.ResponseSender
	log     zerolog.Logger
	tracer  trace.Tracer
}

func NewRestHandler(queries *repository.Queries, index meilisearch.IndexManager) *RestHandler {
	return &RestHandler{
		index:   index,
		queries: queries,
		log:     middleware.GetLogger(),
		res:     response.NewResponseSender(),
		tracer:  otel.Tracer("product-rest-service"),
	}
}

func StartRestServer(ctx context.Context, port string, queries *repository.Queries, index meilisearch.IndexManager, authClient auth.AuthServiceClient, wg *sync.WaitGroup) {
	defer wg.Done()

	r := router.New()

	handler := NewRestHandler(queries, index)
	authMw := middleware.AuthMiddleware(authClient, "product")

	r.GET("/health", handler.healthCheck)
	r.GET("/get", authMw(handler.getProduct))

	server := &fasthttp.Server{
		Handler: tracing.RequestTracingMiddleware(middleware.RequestLoggerMiddleware(r.Handler), "product"),
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
	spanCtx, ok := ctx.UserValue("tracing_context").(context.Context)
	if !ok {
		spanCtx = context.Background()
	}

	query := ctx.QueryArgs().Peek("query")
	if len(query) == 0 {
		h.res.SendError(ctx, fasthttp.StatusBadRequest, "nothing to search")
		return
	}

	user_id := middleware.GetUserIDFromCtx(ctx)
	if user_id == "" {
		h.log.Warn().Msg("no user was found")
	} else {
		h.log.Info().Str("user_id", user_id).Msg("user found")
	}

	_, searchSpan := h.tracer.Start(spanCtx, "meilisearch.Search", trace.WithAttributes(attribute.String("query", string(query))))
	searchRes, err := h.index.Search(string(query), &meilisearch.SearchRequest{
		Limit: 10,
	})
	if err != nil {
		searchSpan.RecordError(err)
		searchSpan.SetStatus(codes.Error, "search failed")
		searchSpan.End()
		h.res.SendError(ctx, fasthttp.StatusInternalServerError, "failed to get products from the search")
		return
	}
	searchSpan.SetAttributes(attribute.Int("hits_count", len(searchRes.Hits)))
	searchSpan.End()

	h.res.SendSuccess(ctx, fasthttp.StatusOK, searchRes.Hits)
}

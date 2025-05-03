package middleware

import (
	"context"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var once sync.Once
var log zerolog.Logger

const (
	ServiceNameEnvVar = "SERVICE_NAME"
)

func RequestLoggerMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		l := GetLogger()

		spanCtx, ok := ctx.UserValue("tracing_context").(context.Context)
		var traceID, spanID string
		if ok {
			span := trace.SpanFromContext(spanCtx)
			traceID = span.SpanContext().TraceID().String()
			spanID = span.SpanContext().SpanID().String()
		}

		defer func() {
			logEvent := l.Info().
				Str("method", string(ctx.Method())).
				Str("path", string(ctx.Path())).
				Dur("elapsed_ms", time.Since(start))

			if traceID != "" {
				logEvent = logEvent.
					Str("trace_id", traceID).
					Str("span_id", spanID)
			}
			logEvent.Msg("rest request")
		}()
		next(ctx)
	}
}

func UnaryServerLoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		l := GetLogger()

		clientIP := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		span := trace.SpanFromContext(ctx)
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()

		resp, err := handler(ctx, req)

		statusCode := codes.OK
		if err != nil {
			statusCode = status.Code(err)
		}

		method := path.Base(info.FullMethod)

		logEvent := l.Info().
			Str("type", "unary").
			Str("method", method).
			Str("full_method", info.FullMethod).
			Str("client_ip", clientIP).
			Str("status", statusCode.String()).
			Dur("elapsed_ms", time.Since(start))

		if traceID != "00000000000000000000000000000000" {
			logEvent = logEvent.
				Str("trace_id", traceID).
				Str("span_id", spanID)
		}

		logEvent.Msg("gRPC request")

		return resp, err
	}
}

func GetLogger() zerolog.Logger {
	once.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		logLevel := zerolog.DebugLevel
		var output io.Writer = os.Stdout

		serviceName := os.Getenv(ServiceNameEnvVar)
		if serviceName == "" {
			serviceName = "unknown-service"
		}

		log = zerolog.New(output).
			Level(zerolog.Level(logLevel)).
			With().
			Timestamp().
			Str("service", serviceName).
			Logger()
	})

	return log
}

func SetServiceName(name string) {
	os.Setenv(ServiceNameEnvVar, name)
}

package middleware

import (
	"context"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var once sync.Once
var log zerolog.Logger

const (
	ServiceNameEnvVar = "SERVICE_NAME"
	RequestIDHeader   = "X-Request-ID"
	RequestIDKey      = "request_id"
)

func RequestID(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		requestID := string(ctx.Request.Header.Peek(RequestIDHeader))
		if requestID == "" {
			requestID = uuid.New().String()
			ctx.Request.Header.Set(RequestIDHeader, requestID)
		}
		ctx.Response.Header.Set(RequestIDHeader, requestID)
		ctx.SetUserValue(RequestIDKey, requestID)
		next(ctx)
	}
}

func RequestLogger(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		l := GetLogger()
		requestID, ok := ctx.UserValue(RequestIDKey).(string)
		if !ok {
			requestID = "unknown"
		}
		defer func() {
			l.Info().
				Str("request_id", requestID).
				Str("method", string(ctx.Method())).
				Str("path", string(ctx.Path())).
				Dur("elapsed_ms", time.Since(start)).
				Msg("rest request")
		}()
		next(ctx)
	}
}

func GRPCRequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		requestID := ""

		if ok {
			if ids := md.Get(RequestIDHeader); len(ids) > 0 {
				requestID = ids[0]
			}
		}

		if requestID == "" {
			requestID = uuid.New().String()
			md = metadata.Join(md, metadata.Pairs(RequestIDHeader, requestID))
			ctx = metadata.NewIncomingContext(ctx, md)
		}

		header := metadata.Pairs(RequestIDHeader, requestID)
		grpc.SetHeader(ctx, header)

		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		return handler(ctx, req)
	}
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		l := GetLogger()

		clientIP := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		requestID := "unknown"
		if id, ok := ctx.Value(RequestIDKey).(string); ok {
			requestID = id
		} else {
			if md, ok := metadata.FromIncomingContext(ctx); ok {
				if ids := md.Get(RequestIDHeader); len(ids) > 0 {
					requestID = ids[0]
				}
			}
		}

		resp, err := handler(ctx, req)

		statusCode := codes.OK
		if err != nil {
			statusCode = status.Code(err)
		}

		method := path.Base(info.FullMethod)

		l.Info().
			Str("request_id", requestID).
			Str("type", "unary").
			Str("method", method).
			Str("full_method", info.FullMethod).
			Str("client_ip", clientIP).
			Str("status", statusCode.String()).
			Dur("elapsed_ms", time.Since(start)).
			Msg("gRPC request")

		return resp, err
	}
}

func GetRequestID(ctx *fasthttp.RequestCtx) string {
	if requestID, ok := ctx.UserValue(RequestIDKey).(string); ok {
		return requestID
	}
	return "unknown"
}

func GetRequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ids := md.Get(RequestIDHeader); len(ids) > 0 {
			return ids[0]
		}
	}
	return "unknown"
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

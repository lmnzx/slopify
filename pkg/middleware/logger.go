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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var once sync.Once
var log zerolog.Logger

func RequestLogger(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		l := GetLogger()
		defer func() {
			l.Info().Str("method", string(ctx.Method())).Str("path", string(ctx.Path())).Dur("elasped_ms", time.Since(start)).Msg("rest request")
		}()
		next(ctx)
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

		resp, err := handler(ctx, req)

		statusCode := codes.OK
		if err != nil {
			statusCode = status.Code(err)
		}

		method := path.Base(info.FullMethod)

		l.Info().
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

func GetLogger() zerolog.Logger {
	once.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		logLevel := zerolog.DebugLevel

		var output io.Writer = os.Stdout

		log = zerolog.New(output).
			Level(zerolog.Level(logLevel)).
			With().
			Timestamp().
			Logger()
	})

	return log
}

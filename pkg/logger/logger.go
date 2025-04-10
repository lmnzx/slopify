package logger

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/valyala/fasthttp"
)

var once sync.Once
var log zerolog.Logger

func RequestLogger(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		l := Get()
		defer func() {
			l.Info().Str("method", string(ctx.Method())).Str("path", string(ctx.Path())).Dur("elasped_ms", time.Since(start)).Msg("incoming request")
		}()
		next(ctx)
	}
}

func Get() zerolog.Logger {
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

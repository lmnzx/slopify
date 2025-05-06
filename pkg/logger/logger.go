package logger

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var once sync.Once
var log zerolog.Logger

const (
	ServiceNameEnvVar = "SERVICE_NAME"
)

func GetLogger() zerolog.Logger {
	once.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		var output io.Writer = os.Stdout

		serviceName := os.Getenv(ServiceNameEnvVar)
		if serviceName == "" {
			serviceName = "unknown-service"
		}

		log = zerolog.New(output).
			Level(zerolog.TraceLevel).
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

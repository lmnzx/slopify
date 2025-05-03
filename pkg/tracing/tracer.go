package tracing

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func InitTracer(serviceName string, serviceVersion string, collectorURL string, log zerolog.Logger) (func(), error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(collectorURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptrace.New(
		ctx,
		otlptracegrpc.NewClient(
			otlptracegrpc.WithGRPCConn(conn),
		),
	)
	if err != nil {
		return nil, err
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(traceProvider)

	// Set global propagator for distributed tracing context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceProvider.Shutdown(ctx); err != nil {
			log.Debug().Err(err).Msg("failed to shutdown tracer provider")
		}
	}, nil
}

func RequestTracingMiddleware(next fasthttp.RequestHandler, serviceName string) fasthttp.RequestHandler {
	tracer := otel.Tracer(serviceName)

	return func(ctx *fasthttp.RequestCtx) {
		carrier := propagation.HeaderCarrier{}
		ctx.Request.Header.VisitAll(func(key, value []byte) {
			carrier.Set(string(key), string(value))
		})

		ctxWithSpan := context.Background()
		prop := otel.GetTextMapPropagator()
		ctxWithSpan = prop.Extract(ctxWithSpan, carrier)

		spanCtx, span := tracer.Start(
			ctxWithSpan,
			string(ctx.Method())+" "+string(ctx.Path()),
			trace.WithAttributes(
				attribute.String("http.method", string(ctx.Method())),
				attribute.String("http.url", string(ctx.URI().FullURI())),
				attribute.String("http.host", string(ctx.Host())),
				attribute.String("http.client_ip", ctx.RemoteIP().String()),
			),
		)
		defer span.End()

		ctx.SetUserValue("tracing_context", spanCtx)

		next(ctx)

		span.SetAttributes(
			attribute.Int("http.status_code", ctx.Response.StatusCode()),
			attribute.Int("http.response_content_length", len(ctx.Response.Body())),
		)
	}
}

func UnaryServerTracingInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	tracer := otel.Tracer(serviceName)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		spanCtx, span := tracer.Start(
			ctx,
			info.FullMethod,
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.method", info.FullMethod),
				attribute.String("rpc.service", serviceName),
			),
		)
		defer span.End()

		resp, err := handler(spanCtx, req)

		if err != nil {
			span.RecordError(err)
		}

		return resp, err
	}
}

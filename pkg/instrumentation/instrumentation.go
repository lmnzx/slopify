package instrumentation

import (
	"context"
	"fmt"
	"time"

	"github.com/lmnzx/slopify/pkg/logger"
	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
)

type InstrumentationConfig struct {
	ServiceName     string
	ServiceVersion  string
	CollectorURL    string
	MetricsInterval time.Duration
}

var (
	restRequestCounter  metric.Int64Counter
	restRequestDuration metric.Float64Histogram
	restActiveRequests  metric.Int64UpDownCounter
	restResponseSize    metric.Int64Histogram
	grpcRequestCounter  metric.Int64Counter
	grpcRequestDuration metric.Float64Histogram
	grpcActiveRequests  metric.Int64UpDownCounter
)

func Init(config InstrumentationConfig) (func(), error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithContainer(),
		resource.WithProcess(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	conn, err := grpc.NewClient(config.CollectorURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	// init tracing
	traceExporter, err := otlptrace.New(
		ctx,
		otlptracegrpc.NewClient(
			otlptracegrpc.WithGRPCConn(conn),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(traceProvider)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// init metric
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricExporter, sdkmetric.WithInterval(config.MetricsInterval),
			),
		),
		sdkmetric.WithResource(res),
		sdkmetric.WithView(
			sdkmetric.NewView(
				sdkmetric.Instrument{Kind: sdkmetric.InstrumentKindHistogram, Name: "http.request.duration"},
				sdkmetric.Stream{
					Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
						Boundaries: []float64{1, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					},
				},
			),
		),
	)
	otel.SetMeterProvider(meterProvider)

	// custom meter
	meter := meterProvider.Meter(config.ServiceName)
	var err1, err2, err3, err4 error
	restRequestCounter, err1 = meter.Int64Counter(
		"http.request.count",
		metric.WithDescription("Number of HTTP requests"),
		metric.WithUnit("{request}"),
	)

	restRequestDuration, err2 = meter.Float64Histogram(
		"http.request.duration",
		metric.WithDescription("Duration of HTTP requests in milliseconds"),
		metric.WithUnit("ms"),
	)

	restActiveRequests, err3 = meter.Int64UpDownCounter(
		"http.request.active",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)

	restResponseSize, err4 = meter.Int64Histogram(
		"http.response.size",
		metric.WithDescription("Size of HTTP responses in bytes"),
		metric.WithUnit("By"),
	)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return nil, fmt.Errorf("failed to create metrics: %v, %v, %v, %v", err1, err2, err3, err4)
	}

	grpcRequestCounter, err1 = meter.Int64Counter(
		"grpc.request.count",
		metric.WithDescription("Number of gRPC requests"),
		metric.WithUnit("{request}"),
	)

	grpcRequestDuration, err2 = meter.Float64Histogram(
		"grpc.request.duration",
		metric.WithDescription("Duration of gRPC requests in milliseconds"),
		metric.WithUnit("ms"),
	)

	grpcActiveRequests, err3 = meter.Int64UpDownCounter(
		"grpc.request.active",
		metric.WithDescription("Number of active gRPC requests"),
		metric.WithUnit("{request}"),
	)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, fmt.Errorf("failed to create metrics: %v, %v, %v", err1, err2, err3)
	}

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log := logger.GetLogger()
		if err := traceProvider.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("failed to shutdown tracer provider")
		}

		if err := meterProvider.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("failed to shutdown meter provider")
		}

	}, nil
}

func RequestInstrumentationMiddleware(next fasthttp.RequestHandler, serviceName string) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		tracer := otel.Tracer(serviceName)

		carrier := propagation.HeaderCarrier{}
		ctx.Request.Header.VisitAll(func(key, value []byte) {
			carrier.Set(string(key), string(value))
		})

		ctxWithSpan := context.Background()
		prop := otel.GetTextMapPropagator()
		ctxWithSpan = prop.Extract(ctxWithSpan, carrier)

		restActiveRequests.Add(ctxWithSpan, 1)
		startTime := time.Now()
		spanCtx, span := tracer.Start(
			ctxWithSpan,
			string(ctx.Method())+" "+string(ctx.Path()),
			trace.WithAttributes(
				attribute.String("http.method", string(ctx.Method())),
				attribute.String("http.url", string(ctx.URI().FullURI())),
				attribute.String("http.host", string(ctx.Host())),
				attribute.String("http.client_ip", ctx.RemoteIP().String()),
				attribute.Int("http.request_size", len(ctx.Request.Body())),
			),
			trace.WithTimestamp(startTime),
		)
		ctx.SetUserValue("tracing_context", spanCtx)

		next(ctx)

		span.End(trace.WithTimestamp(time.Now()))

		duration := time.Since(startTime)
		statusCode := ctx.Response.StatusCode()
		responseBodySize := len(ctx.Response.Body())
		traceID := span.SpanContext().TraceID().String()

		log := logger.GetLogger()

		responseLog := log.With().
			Str("method", string(ctx.Method())).
			Str("path", string(ctx.Path())).
			Str("remote_ip", ctx.RemoteIP().String()).
			Dur("elapsed_ms", duration).
			Str("trace_id", traceID).
			Int("status_code", statusCode).
			Int("response_body_size", responseBodySize).
			Logger()

		span.SetAttributes(
			attribute.Int("http.status_code", statusCode),
			attribute.Int("http.response_content_length", responseBodySize),
		)

		if statusCode >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("Request failed with status code %d", statusCode))
			if errValue := ctx.UserValue("error"); errValue != nil {
				if err, ok := errValue.(error); ok {
					span.RecordError(err)
					span.SetAttributes(attribute.String("error.message", err.Error()))
				}
			}
			responseLog.Error().Msg("request completed with error")
		} else {
			span.SetStatus(codes.Ok, "")
			responseLog.Info().Msg("request completed successfully")
		}

		metricAttrs := []attribute.KeyValue{
			attribute.String("http.method", string(ctx.Method())),
			attribute.String("http.route", string(ctx.Path())),
			attribute.Int("http.status_code", statusCode),
		}

		restRequestCounter.Add(ctxWithSpan, 1, metric.WithAttributes(metricAttrs...))
		restRequestDuration.Record(ctxWithSpan, float64(duration.Milliseconds()), metric.WithAttributes(metricAttrs...))
		restResponseSize.Record(ctxWithSpan, int64(responseBodySize), metric.WithAttributes(metricAttrs...))
		restActiveRequests.Add(ctxWithSpan, -1)
	}
}

func UnaryInstrumentationMiddleware(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		tracer := otel.Tracer(serviceName)

		clientIP := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		grpcActiveRequests.Add(ctx, 1, metric.WithAttributes(
			attribute.String("rpc.service", serviceName),
			attribute.String("rpc.method", info.FullMethod),
		))
		startTime := time.Now()
		spanCtx, span := tracer.Start(
			ctx,
			info.FullMethod,
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.method", info.FullMethod),
				attribute.String("rpc.service", serviceName),
				attribute.String("rpc.client_ip", clientIP),
			),
			trace.WithTimestamp(startTime),
		)

		resp, err := handler(spanCtx, req)

		span.End(trace.WithTimestamp(time.Now()))

		duration := time.Since(startTime)
		traceID := span.SpanContext().TraceID().String()

		statusCode := "OK"
		if err != nil {
			statusCode = "ERROR"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		log := logger.GetLogger()

		responseLog := log.With().
			Str("method", info.FullMethod).
			Str("client_ip", clientIP).
			Dur("elapsed_ms", duration).
			Str("trace_id", traceID).
			Str("status", statusCode).
			Logger()

		grpcActiveRequests.Add(ctx, -1, metric.WithAttributes(
			attribute.String("rpc.service", serviceName),
			attribute.String("rpc.method", info.FullMethod),
		))

		attrs := []attribute.KeyValue{
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", serviceName),
			attribute.String("rpc.method", info.FullMethod),
			attribute.String("rpc.status", statusCode),
		}

		grpcRequestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		grpcRequestDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))

		if err != nil {
			responseLog.Error().Msg("request completed with error")
		} else {
			responseLog.Info().Msg("request completed successfully")
		}

		return resp, err
	}
}

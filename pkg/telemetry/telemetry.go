package telemetry

//
// import (
// 	"context"
// 	"fmt"
//
// 	"go.opentelemetry.io/otel"
// 	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
// 	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
// 	"go.opentelemetry.io/otel/sdk/metric"
// 	"go.opentelemetry.io/otel/sdk/resource"
// 	"go.opentelemetry.io/otel/sdk/trace"
// 	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
// )
//
// func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
// 	res, err := resource.New(ctx,
// 		resource.WithAttributes(
// 			semconv.ServiceNameKey.String(serviceName),
// 		),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create resource: %w", err)
// 	}
//
// 	// Tracer
// 	traceExp, err := otlptracehttp.New(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
// 	}
//
// 	tracerProvider := trace.NewTracerProvider(
// 		trace.WithBatcher(traceExp),
// 		trace.WithResource(res),
// 	)
// 	otel.SetTracerProvider(tracerProvider)
//
// 	// Metrics
// 	metricExp, err := otlpmetrichttp.New(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
// 	}
//
// 	meterProvider := metric.NewMeterProvider(
// 		metric.WithReader(metric.NewPeriodicReader(metricExp)),
// 		metric.WithResource(res),
// 	)
// 	otel.SetMeterProvider(meterProvider)
//
// 	return func(ctx context.Context) error {
// 		if err := tracerProvider.Shutdown(ctx); err != nil {
// 			return err
// 		}
// 		return meterProvider.Shutdown(ctx)
// 	}, nil
// }

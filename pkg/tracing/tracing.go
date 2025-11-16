package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"go.uber.org/zap"
)

var tracer trace.Tracer

// InitTracer initializes the OpenTelemetry tracer provider
func InitTracer(serviceName, serviceVersion, environment, alloyEndpoint string) (func(context.Context) error, error) {
	if alloyEndpoint == "" {
		logger.Info("Tracing disabled: ALLOY_ENDPOINT not set")
		return func(context.Context) error { return nil }, nil
	}

	logger.Info("Initializing OpenTelemetry tracer",
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("environment", environment),
		zap.String("endpoint", alloyEndpoint))

	// Create OTLP exporter
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(alloyEndpoint),
		otlptracegrpc.WithInsecure(), // Alloy is on internal network
		otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.DeploymentEnvironment(environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Sample all traces in production
	)

	// Set global tracer provider and propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Get tracer instance
	tracer = tp.Tracer(serviceName)

	logger.Info("OpenTelemetry tracer initialized successfully")

	// Return shutdown function
	return tp.Shutdown, nil
}

// Tracer returns the global tracer instance
func Tracer() trace.Tracer {
	return tracer
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, spanName string) (context.Context, trace.Span) {
	if tracer == nil {
		// Return no-op span if tracer not initialized
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracer.Start(ctx, spanName)
}

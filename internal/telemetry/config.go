package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// Config contains OpenTelemetry configuration
type Config struct {
	// Enable tracing
	Enabled bool

	// Service name for traces
	ServiceName string

	// OTLP endpoint (e.g., localhost:4317)
	Endpoint string

	// Sampling configuration
	SamplingRatio float64

	// Connection timeout
	Timeout time.Duration

	// Additional resource attributes
	Attributes map[string]string
}

// DefaultConfig returns default telemetry configuration
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		ServiceName:   "engram",
		Endpoint:      "localhost:4317",
		SamplingRatio: 0.1,
		Timeout:       5 * time.Second,
		Attributes:    map[string]string{},
	}
}

// Setup initializes OpenTelemetry
func Setup(ctx context.Context, config Config) (shutdown func(context.Context) error, err error) {
	if !config.Enabled {
		// Return a no-op shutdown function when telemetry is disabled
		return func(context.Context) error { return nil }, nil
	}

	logger := log.With().Str("component", "telemetry").Logger()
	logger.Info().Msg("Setting up OpenTelemetry tracing")

	// Create OTLP exporter
	var exporter *otlptrace.Exporter

	secureOption := otlptracegrpc.WithInsecure()
	exporter, err = otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(config.Endpoint),
		secureOption,
		otlptracegrpc.WithTimeout(config.Timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Create resource with service information and any additional attributes
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(config.ServiceName),
	}
	
	// Add custom attributes
	for k, v := range config.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(attrs...),
		resource.WithProcessPID(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider with the exporter
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(config.SamplingRatio),
	)

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	
	// Set global trace provider and propagator
	otel.SetTracerProvider(traceProvider)
	
	// Use W3C context propagator
	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	// Return a function to shut down the tracer provider
	return func(ctx context.Context) error {
		logger.Info().Msg("Shutting down OpenTelemetry tracing")
		return traceProvider.Shutdown(ctx)
	}, nil
}

// Tracer returns a named tracer from the global provider
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
package telemetry

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

// Init initializes OpenTelemetry providers (tracing, metrics, logging) based on configuration.
// If the OTLP endpoint is not configured, returns a noop shutdown function (telemetry disabled).
// This ensures zero overhead when observability is not needed.
//
// Returns:
//   - shutdown: Function to gracefully shutdown telemetry (flushes pending data)
//   - error: Initialization error (only if endpoint is configured)
func Init(ctx context.Context, cfg config.ObservabilityConfig) (shutdown func(context.Context) error, err error) {
	// If no OTLP endpoint configured, telemetry is disabled (noop mode)
	if cfg.OTLPEndpoint == "" {
		log.Println("Telemetry disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)")
		return func(context.Context) error { return nil }, nil
	}

	log.Printf("Initializing OpenTelemetry: endpoint=%s, protocol=%s, service=%s",
		cfg.OTLPEndpoint, cfg.OTLPProtocol, cfg.ServiceName)

	// Create resource with service identification attributes
	res, err := newResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	// Initialize trace provider
	tracerProvider, err := newTracerProvider(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracer provider: %w", err)
	}

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Setup W3C Trace Context propagation for distributed tracing
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	log.Println("OpenTelemetry initialized successfully")

	// Return shutdown function that flushes all providers
	return func(ctx context.Context) error {
		log.Println("Shutting down OpenTelemetry...")
		if err := tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown tracer provider: %w", err)
		}
		log.Println("OpenTelemetry shutdown complete")
		return nil
	}, nil
}

// newResource creates an OTEL resource with service identification attributes.
func newResource(cfg config.ObservabilityConfig) (*resource.Resource, error) {
	// Default resource includes basic runtime attributes
	// Merge with custom service identification
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
}

// newTracerProvider creates a TracerProvider with OTLP HTTP exporter.
func newTracerProvider(ctx context.Context, res *resource.Resource, cfg config.ObservabilityConfig) (*sdktrace.TracerProvider, error) {
	// Configure OTLP exporter options
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
	}

	// Add protocol-specific options
	if cfg.OTLPProtocol == "http/protobuf" {
		// Default protocol, no additional options needed
	} else if cfg.OTLPProtocol == "grpc" {
		// Note: Would need to use otlptracegrpc instead of otlptracehttp
		return nil, fmt.Errorf("gRPC protocol not implemented yet, use http/protobuf")
	}

	// Add insecure option for development (no TLS)
	if cfg.OTLPInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Create tracer provider with batch span processor (performance)
	// Batch processor aggregates spans before export to reduce network overhead
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	return tp, nil
}

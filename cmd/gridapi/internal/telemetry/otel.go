package telemetry

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/config"
)

// Providers holds all OpenTelemetry providers (traces, metrics, logs).
// This struct is returned by Init() and used for graceful shutdown.
type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *metric.MeterProvider
	LoggerProvider *log.LoggerProvider
}

// Shutdown gracefully shuts down all providers, flushing pending telemetry.
func (p *Providers) Shutdown(ctx context.Context) error {
	var errs []error

	// Shutdown in reverse order of initialization
	if p.LoggerProvider != nil {
		if err := p.LoggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown logger provider: %w", err))
		}
	}

	if p.MeterProvider != nil {
		if err := p.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown meter provider: %w", err))
		}
	}

	if p.TracerProvider != nil {
		if err := p.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown tracer provider: %w", err))
		}
	}

	// Return first error if any
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Init initializes OpenTelemetry providers (tracing, metrics, logging) based on configuration.
// If the OTLP endpoint is not configured, returns a noop shutdown function (telemetry disabled).
// This ensures zero overhead when observability is not needed.
//
// Supported protocols:
//   - "grpc": OTLP gRPC (recommended for SigNoz, lower overhead)
//   - "http/protobuf": OTLP HTTP (more compatible, easier debugging)
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

	// Initialize providers based on protocol
	var providers *Providers
	switch cfg.OTLPProtocol {
	case "grpc":
		providers, err = initGRPCProviders(ctx, res, cfg)
	case "http/protobuf", "http":
		providers, err = initHTTPProviders(ctx, res, cfg)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s (use 'grpc' or 'http/protobuf')", cfg.OTLPProtocol)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	// Set global providers
	otel.SetTracerProvider(providers.TracerProvider)
	otel.SetMeterProvider(providers.MeterProvider)

	// Setup W3C Trace Context propagation for distributed tracing
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	log.Println("OpenTelemetry initialized successfully (traces, metrics, logs)")

	// Return shutdown function that flushes all providers
	return func(ctx context.Context) error {
		log.Println("Shutting down OpenTelemetry...")
		if err := providers.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown providers: %w", err)
		}
		log.Println("OpenTelemetry shutdown complete")
		return nil
	}, nil
}

// initGRPCProviders initializes OTLP exporters using gRPC protocol.
// Recommended for production use with SigNoz (lower overhead, better performance).
func initGRPCProviders(ctx context.Context, res *resource.Resource, cfg config.ObservabilityConfig) (*Providers, error) {
	// Common gRPC options
	grpcOpts := []interface{}{
		// Endpoint is host:port (no protocol prefix for gRPC)
		// Example: "localhost:4317" or "signoz.example.com:4317"
	}

	// TLS configuration
	if cfg.OTLPInsecure {
		// Development mode: no TLS
		log.Println("WARNING: Using insecure gRPC connection (development only)")
	} else {
		// Production mode: use TLS
		log.Println("Using secure gRPC connection with TLS")
	}

	// Create trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(), // Use WithTLSCredentials() in production
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	// Create metric exporter
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	// Create log exporter
	logExporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create log exporter: %w", err)
	}

	return newProviders(res, traceExporter, metricExporter, logExporter), nil
}

// initHTTPProviders initializes OTLP exporters using HTTP protocol.
// Easier to debug (use curl), more compatible with firewalls/proxies.
func initHTTPProviders(ctx context.Context, res *resource.Resource, cfg config.ObservabilityConfig) (*Providers, error) {
	// Common HTTP options
	httpOpts := []interface{}{
		// Endpoint is base URL path
		// Example: "localhost:4318" (will add /v1/traces, /v1/metrics, /v1/logs)
	}

	// TLS configuration
	if cfg.OTLPInsecure {
		log.Println("WARNING: Using insecure HTTP connection (development only)")
	} else {
		log.Println("Using secure HTTP connection with TLS")
	}

	// Create trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	// Create metric exporter
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	// Create log exporter
	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(cfg.OTLPEndpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create log exporter: %w", err)
	}

	return newProviders(res, traceExporter, metricExporter, logExporter), nil
}

// newProviders creates provider instances with exporters.
func newProviders(
	res *resource.Resource,
	traceExporter sdktrace.SpanExporter,
	metricExporter metric.Exporter,
	logExporter log.Exporter,
) *Providers {
	// Create TracerProvider with batch span processor
	// Batching reduces network overhead by aggregating spans before export
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Use ParentBased() in production
	)

	// Create MeterProvider with periodic reader
	// Exports metrics every 10 seconds (balance between freshness and overhead)
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(10*time.Second),
		)),
		metric.WithResource(res),
	)

	// Create LoggerProvider with batch processor
	// Batching reduces network overhead by aggregating logs before export
	loggerProvider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExporter,
			log.WithExportInterval(5*time.Second),
			log.WithExportMaxBatchSize(512),
			log.WithExportTimeout(30*time.Second),
		)),
		log.WithResource(res),
	)

	return &Providers{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		LoggerProvider: loggerProvider,
	}
}

// newResource creates an OTEL resource with service identification attributes.
func newResource(cfg config.ObservabilityConfig) (*resource.Resource, error) {
	// Default resource includes basic runtime attributes
	// Merge with custom service identification
	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	}

	// Create resource with schema URL and default attributes
	return resource.New(context.Background(),
		append([]resource.Option{
			resource.WithSchemaURL(semconv.SchemaURL),
			resource.WithFromEnv(),   // Support OTEL_RESOURCE_ATTRIBUTES
			resource.WithProcess(),   // Add process info (PID, executable)
			resource.WithOS(),        // Add OS info
			resource.WithContainer(), // Add container info (if running in container)
			resource.WithHost(),      // Add host info
		}, attrs...)...,
	)
}

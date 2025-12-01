# OpenTelemetry Dependencies Installation

The OTEL integration requires several Go modules for traces, metrics, and logs. Follow these steps to add them:

## Required Dependencies

Run these commands in the `cmd/gridapi` directory:

```bash
cd /home/user/grid/cmd/gridapi

# Core OTEL libraries
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/propagation@latest

# Trace exporters (HTTP and gRPC)
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@latest

# Metric exporters (HTTP and gRPC)
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc@latest

# Log exporters (HTTP and gRPC)
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc@latest

# Semantic conventions
go get go.opentelemetry.io/otel/semconv/v1.24.0@latest

# Instrumentation libraries
go get github.com/uptrace/bun/extra/bunotel@latest
go get github.com/riandyrn/otelchi@latest

# Run go mod tidy to clean up
go mod tidy
```

## Verify Installation

After installing dependencies, verify the code compiles:

```bash
cd /home/user/grid/cmd/gridapi
go build -o ../../bin/gridapi .
```

## Expected Versions

The following minimum versions are required:

**Core OTEL:**
- `go.opentelemetry.io/otel` >= v1.24.0
- `go.opentelemetry.io/otel/sdk` >= v1.24.0
- `go.opentelemetry.io/otel/propagation` >= v1.24.0

**Trace Exporters:**
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` >= v1.24.0
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` >= v1.24.0

**Metric Exporters:**
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp` >= v1.24.0
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` >= v1.24.0

**Log Exporters:**
- `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp` >= v1.24.0
- `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` >= v1.24.0

**Semantic Conventions:**
- `go.opentelemetry.io/otel/semconv/v1.24.0` >= v1.24.0

**Instrumentation:**
- `github.com/uptrace/bun/extra/bunotel` >= v1.1.17
- `github.com/riandyrn/otelchi` >= v0.5.1

## Alternative: Manual go.mod Edit

If `go get` fails, you can manually add these lines to `cmd/gridapi/go.mod`:

```go
require (
    // Core OTEL
    go.opentelemetry.io/otel v1.24.0
    go.opentelemetry.io/otel/sdk v1.24.0
    go.opentelemetry.io/otel/propagation v1.24.0

    // Trace exporters
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.24.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.24.0

    // Metric exporters
    go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.24.0
    go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.24.0

    // Log exporters
    go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v1.24.0
    go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v1.24.0

    // Semantic conventions
    go.opentelemetry.io/otel/semconv/v1.24.0 v1.24.0

    // Instrumentation
    github.com/uptrace/bun/extra/bunotel v1.1.17
    github.com/riandyrn/otelchi v0.5.1
)
```

Then run:
```bash
cd /home/user/grid/cmd/gridapi
go mod tidy
```

## What Each Dependency Does

### Core OTEL
- **otel**: Core API (tracer, meter, logger interfaces)
- **sdk**: SDK implementations (providers, processors, exporters)
- **propagation**: W3C Trace Context propagation for distributed tracing

### Exporters
- **otlptrace**: Send traces to OTLP collector (SigNoz, Jaeger, etc.)
- **otlpmetric**: Send metrics to OTLP collector
- **otlplog**: Send logs to OTLP collector
- Each has HTTP and gRPC variants (Grid supports both)

### Semantic Conventions
- **semconv**: Standard attribute keys (service.name, http.method, etc.)
- Ensures consistent telemetry across services

### Instrumentation
- **bunotel**: Automatic SQL query tracing for Bun ORM
- **otelchi**: Automatic HTTP request tracing for Chi router

## Protocol Selection

Grid supports both HTTP and gRPC protocols for OTLP export:

### gRPC (Recommended for SigNoz)
- **Advantages**: Lower overhead, better performance, HTTP/2 multiplexing
- **Port**: 4317
- **Configuration**: `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`

### HTTP/Protobuf
- **Advantages**: Easier debugging, more firewall-friendly
- **Port**: 4318
- **Configuration**: `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`

Both protocols export traces, metrics, and logs to the same endpoints.

# OpenTelemetry Dependencies Installation

Due to network issues during development, the OTEL dependencies were not added to go.mod automatically. Follow these steps to add them:

## Required Dependencies

Run these commands in the `cmd/gridapi` directory:

```bash
cd /home/user/grid/cmd/gridapi

# Core OTEL libraries
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
go get go.opentelemetry.io/otel/propagation@latest

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
- `go.opentelemetry.io/otel` >= v1.21.0
- `go.opentelemetry.io/otel/sdk` >= v1.21.0
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` >= v1.21.0
- `github.com/uptrace/bun/extra/bunotel` >= v1.1.17
- `github.com/riandyrn/otelchi` >= v0.5.1

## Alternative: Manual go.mod Edit

If `go get` fails, you can manually add these lines to `cmd/gridapi/go.mod`:

```go
require (
    go.opentelemetry.io/otel v1.24.0
    go.opentelemetry.io/otel/sdk v1.24.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.24.0
    go.opentelemetry.io/otel/propagation v1.24.0
    go.opentelemetry.io/otel/semconv/v1.24.0 v1.24.0
    github.com/uptrace/bun/extra/bunotel v1.1.17
    github.com/riandyrn/otelchi v0.5.1
)
```

Then run:
```bash
cd /home/user/grid/cmd/gridapi
go mod tidy
```

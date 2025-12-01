# OpenTelemetry Integration Design for Grid

## Overview

This document defines the design for adding comprehensive observability to Grid using OpenTelemetry (OTEL). The integration will provide:

1. **Distributed Tracing**: Request flows across HTTP → Services → Database
2. **SQL Metrics**: Automatic query instrumentation via Bun OTEL integration
3. **HTTP Metrics**: Request latency, status codes, route-level metrics
4. **Structured Context**: Service-layer business logic telemetry

## Architecture Alignment

Based on Grid's layering architecture (see `cmd/gridapi/layering.md`), OTEL integration will be applied at specific layers:

```
┌─────────────────────────────────────────────────────────┐
│ Layer 8: Commands (CLI)                                 │
│ - Initialize OTEL providers (TracerProvider, etc.)      │
│ - Graceful shutdown on SIGTERM/SIGINT                   │
└─────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────┐
│ Layer 6: Middleware                                      │
│ - Chi OTEL middleware (HTTP tracing, metrics)           │
│ - Propagate W3C Trace Context                           │
│ - Record: latency, status codes, route patterns         │
└─────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────┐
│ Layer 7: Handlers (Server)                              │
│ - Receive context with active span from middleware      │
│ - Record errors as span events                          │
│ - Pass context to services                              │
└─────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────┐
│ Layer 4: Services (Business Logic) ★ PRIMARY TARGET    │
│ - Create child spans for operations                     │
│ - Record: operation name, inputs, outputs, errors       │
│ - Example: "state.CreateState", "iam.Authenticate"      │
│ - Semantic attributes for business context              │
└─────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────┐
│ Layer 3: Repositories (Data Access)                     │
│ - Bun OTEL hook automatically instruments SQL queries   │
│ - NO manual instrumentation needed (handled by Bun)     │
│ - Records: query SQL, duration, errors                  │
└─────────────────────────────────────────────────────────┘
```

### OTEL Integration Rules by Layer

#### ✅ Layer 8: Commands (CLI)
- **Responsibility**: Initialize and shutdown OTEL providers
- **Integration**: Create `internal/telemetry` package for provider setup
- **Pattern**: Conditional initialization (noop if endpoint not configured)

#### ✅ Layer 6: Middleware
- **Responsibility**: HTTP-level tracing and metrics
- **Integration**: Add `otelchi` middleware to router
- **Metrics**: Request count, latency histograms, active requests
- **Tracing**: Create root span for each HTTP request

#### ✅ Layer 7: Handlers
- **Responsibility**: Minimal - just pass context and record errors
- **Pattern**: Handlers receive context from middleware, no manual span creation
- **Error Recording**: Use `span.RecordError(err)` for failed requests

#### ✅ Layer 4: Services (PRIMARY TELEMETRY TARGET)
- **Responsibility**: Business logic telemetry
- **Integration**: Create child spans for key operations
- **Examples**:
  - `state.Service.CreateState()` → span "state.CreateState"
  - `iam.Service.Authenticate()` → span "iam.Authenticate"
  - `dependency.Service.GetDependencyGraph()` → span "dependency.GetDependencyGraph"
- **Attributes**: Add semantic attributes (GUID, logic_id, user_id, etc.)
- **Events**: Record business events (validation failures, policy checks)

#### ✅ Layer 3: Repositories
- **Responsibility**: Automatic SQL instrumentation
- **Integration**: Bun query hook (`bunotel.NewQueryHook`)
- **NO manual work required** - Bun handles all span creation

#### ❌ Layer 2: DB Providers & Migrations
- **No instrumentation needed** - covered by Bun hook

#### ❌ Layer 1: Data Models
- **No instrumentation needed** - pure data structures

## OTEL Components

### 1. TracerProvider (Distributed Tracing)

```go
// Resource: Service identification
resource.NewWithAttributes(
    semconv.ServiceName("gridapi"),
    semconv.ServiceVersion("1.0.0"),
    semconv.DeploymentEnvironment(env),
)

// Exporter: OTLP HTTP to collector
otlptracehttp.New(ctx,
    otlptracehttp.WithEndpoint(endpoint),
    otlptracehttp.WithInsecure(), // Development only
)

// Provider: Batched export for performance
sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exporter),
    sdktrace.WithResource(res),
)
```

### 2. MeterProvider (Metrics) - Future

For now, focus on tracing. Metrics can be added in Phase 2:
- HTTP request count by route/status
- Database connection pool stats
- Service operation durations

### 3. Propagation (W3C Trace Context)

```go
// Ensures trace context flows across service boundaries
otel.SetTextMapPropagator(
    propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ),
)
```

## Configuration

### New Config Fields

```go
// cmd/gridapi/internal/config/config.go
type Config struct {
    // ... existing fields

    // Observability configuration
    Observability ObservabilityConfig
}

type ObservabilityConfig struct {
    // OTLP exporter endpoint (e.g., "localhost:4318" for HTTP)
    // If empty, telemetry is disabled (noop providers)
    OTLPEndpoint string

    // OTLP protocol: "http/protobuf" or "grpc"
    // Default: "http/protobuf"
    OTLPProtocol string

    // Use insecure connection (development only)
    // Default: false (use TLS in production)
    OTLPInsecure bool

    // Service name for telemetry
    // Default: "gridapi"
    ServiceName string

    // Service version (set from build-time variable)
    ServiceVersion string

    // Deployment environment (e.g., "production", "staging", "dev")
    // Default: "development"
    Environment string
}
```

### Environment Variables

```bash
# OTEL configuration (standard OTLP environment variables)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318   # Required to enable telemetry
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf           # Optional: default http/protobuf
OTEL_EXPORTER_OTLP_INSECURE=true                    # Optional: default false

# Service identification
OTEL_SERVICE_NAME=gridapi                            # Optional: default "gridapi"
OTEL_SERVICE_VERSION=1.0.0                           # Optional: from build-time variable
OTEL_DEPLOYMENT_ENVIRONMENT=production               # Optional: default "development"

# Optional: Authentication headers for collector
OTEL_EXPORTER_OTLP_HEADERS=authorization=Bearer token
```

## Implementation Components

### 1. Telemetry Package (`cmd/gridapi/internal/telemetry/`)

```
internal/telemetry/
├── otel.go          # Provider initialization and shutdown
└── noop.go          # Noop implementation when disabled
```

**Responsibilities**:
- Initialize TracerProvider when endpoint configured
- Return noop shutdown function when endpoint empty
- Setup resource attributes (service name, version, environment)
- Configure propagators (W3C Trace Context)
- Graceful shutdown with context timeout

### 2. Service Layer Instrumentation

**Pattern**:
```go
func (s *Service) CreateState(ctx context.Context, guid, logicID string) error {
    tracer := otel.Tracer("gridapi/services/state")
    ctx, span := tracer.Start(ctx, "state.CreateState",
        trace.WithAttributes(
            attribute.String("state.guid", guid),
            attribute.String("state.logic_id", logicID),
        ),
    )
    defer span.End()

    // Business logic here
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return err
    }

    return nil
}
```

**Services to Instrument**:
- `internal/services/state/service.go`
- `internal/services/iam/iam_service.go`
- `internal/services/dependency/service.go`
- `internal/services/graph/service.go`
- `internal/services/inference/service.go`
- `internal/services/tfstate/service.go`
- `internal/services/validation/validator.go`

### 3. Bun Database Integration

```go
// cmd/gridapi/cmd/serve.go
import "github.com/uptrace/bun/extra/bunotel"

// After db initialization
db.AddQueryHook(bunotel.NewQueryHook(
    bunotel.WithDBName("grid"),
    bunotel.WithFormattedQueries(true),  // Include SQL in spans
))
```

### 4. Chi HTTP Middleware

```go
// cmd/gridapi/cmd/serve.go
import "github.com/riandyrn/otelchi"

// Router setup
r := chi.NewRouter()
r.Use(otelchi.Middleware("gridapi",
    otelchi.WithChiRoutes(r),  // Use route patterns, not raw paths
))
```

## Conditional Enablement

### Design Principle
- **Default**: Telemetry disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)
- **Zero Overhead**: Noop providers when disabled (no performance impact)
- **Explicit Opt-In**: Must configure endpoint to enable

### Implementation
```go
func Init(ctx context.Context, cfg ObservabilityConfig) (shutdown func(context.Context) error, err error) {
    // If endpoint empty, return noop (telemetry disabled)
    if cfg.OTLPEndpoint == "" {
        log.Println("Telemetry disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)")
        return func(context.Context) error { return nil }, nil
    }

    log.Printf("Initializing telemetry: endpoint=%s", cfg.OTLPEndpoint)

    // Initialize providers...
    return shutdown, nil
}
```

## Semantic Conventions

### Span Names
- Format: `<layer>.<operation>`
- Examples: `state.CreateState`, `iam.Authenticate`, `http.POST /api/states`

### Span Attributes (Services)
```go
// State service
attribute.String("state.guid", guid)
attribute.String("state.logic_id", logicID)
attribute.StringSlice("state.labels", labelKeys)

// IAM service
attribute.String("principal.id", principal.ID)
attribute.String("principal.type", principal.Type)
attribute.StringSlice("principal.roles", roles)

// Dependency service
attribute.String("dependency.source_guid", sourceGUID)
attribute.String("dependency.target_guid", targetGUID)
attribute.String("dependency.edge_type", edgeType)
```

### Span Events
```go
// Business events
span.AddEvent("validation.failed",
    trace.WithAttributes(
        attribute.String("reason", "invalid label format"),
    ),
)

span.AddEvent("policy.check",
    trace.WithAttributes(
        attribute.String("action", "state:create"),
        attribute.Bool("allowed", true),
    ),
)
```

## Testing Strategy

### Unit Tests
- Mock OTEL tracer for service tests
- Verify span names and attributes
- Use `go.opentelemetry.io/otel/sdk/trace/tracetest`

### Integration Tests
- Use OTLP exporter with in-memory backend
- Verify end-to-end trace propagation (HTTP → Service → DB)
- Validate span hierarchy

### Manual Testing
- Run local OTEL collector (Docker)
- Export to Jaeger for visualization
- Validate trace completeness

## Dependencies

```bash
# Core OTEL
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/sdk/trace@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest

# Instrumentation
go get github.com/uptrace/bun/extra/bunotel@latest
go get github.com/riandyrn/otelchi@latest

# Semantic conventions
go get go.opentelemetry.io/otel/semconv/v1.24.0@latest
```

## Rollout Plan

### Phase 1: Foundation (This PR)
1. Add telemetry configuration to config package
2. Create telemetry initialization package
3. Add Bun OTEL hook (SQL tracing)
4. Add Chi OTEL middleware (HTTP tracing)
5. Wire up in serve.go with conditional enablement

### Phase 2: Service Instrumentation (Follow-up PR)
1. Add tracing to state service
2. Add tracing to IAM service
3. Add tracing to dependency service
4. Add semantic attributes and events

### Phase 3: Metrics (Future)
1. Add MeterProvider initialization
2. HTTP metrics (request count, latency)
3. Database metrics (connection pool, query stats)
4. Custom business metrics (states created, auth failures)

## Success Metrics

- ✅ HTTP request traces show full flow: Request → Handler → Service → DB
- ✅ SQL queries appear as child spans with formatted SQL
- ✅ Service-level spans include business context (GUID, logic_id)
- ✅ Zero overhead when OTEL disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)
- ✅ Graceful shutdown flushes pending telemetry
- ✅ Layering rules maintained (no repository imports in handlers)

## References

- [OpenTelemetry Go Docs](https://opentelemetry.io/docs/languages/go/)
- [Bun Performance Monitoring](https://bun.uptrace.dev/guide/performance-monitoring.html)
- [otelchi GitHub](https://github.com/riandyrn/otelchi)
- [OTLP Exporter Spec](https://opentelemetry.io/docs/specs/otel/protocol/exporter/)

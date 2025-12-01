# Observability with OpenTelemetry

Grid API includes comprehensive observability using OpenTelemetry (OTEL) for distributed tracing, SQL metrics, and HTTP instrumentation.

## Features

### 1. Distributed Tracing
- **HTTP Layer**: Automatic span creation for all HTTP requests via Chi middleware
- **Service Layer**: Business logic instrumentation (state operations, IAM, dependencies)
- **Database Layer**: Automatic SQL query tracing via Bun OTEL hook
- **W3C Trace Context**: Propagation for distributed tracing across services

### 2. Instrumentation Points

#### HTTP Middleware (`otelchi`)
- Creates root span for each HTTP request
- Records: method, route pattern, status code, latency
- Propagates W3C Trace Context headers
- Example span: `GET /api/states/:id`

#### Service Layer
Instrumented operations include:
- **State Service**: `CreateState`, `ListStates`, `GetStateConfig`
- **IAM Service**: `AuthenticateRequest`, `ResolveRoles`
- **Dependency Service**: (future) `CreateEdge`, `GetDependencyGraph`

Each service span includes:
- Operation name (e.g., `state.CreateState`)
- Business context attributes (GUID, logic_id, principal_id, etc.)
- Events for validation failures, policy checks, errors

#### Database Layer (`bunotel`)
- Automatic span for every SQL query
- Records: query SQL, duration, affected rows, errors
- Zero manual work required

## Configuration

### Environment Variables

```bash
# Enable telemetry (required)
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Optional: Protocol (default: http/protobuf)
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf

# Optional: Use insecure connection (dev only, default: false)
export OTEL_EXPORTER_OTLP_INSECURE=true

# Optional: Service identification
export OTEL_SERVICE_NAME=gridapi
export OTEL_SERVICE_VERSION=1.0.0
export OTEL_DEPLOYMENT_ENVIRONMENT=production

# Optional: Authentication headers for collector
export OTEL_EXPORTER_OTLP_HEADERS=authorization=Bearer <token>
```

### Conditional Enablement

Telemetry is **completely disabled** by default. To enable:
1. Set `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable
2. Start the Grid API server

If the endpoint is not set, Grid uses noop providers with **zero overhead**.

## Local Development Setup

### Using Jaeger (All-in-One)

Jaeger includes an OTLP receiver and provides a UI for trace visualization:

```bash
# Start Jaeger with OTLP support
docker run -d --name jaeger \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest

# Configure Grid to export to Jaeger
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_INSECURE=true

# Start Grid API
./bin/gridapi serve

# View traces in Jaeger UI
open http://localhost:16686
```

### Using OTEL Collector + Jaeger

For production-like setup with collector pipeline:

```yaml
# docker-compose.yml
version: '3'
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # Jaeger UI
      - "14250:14250"  # Jaeger gRPC receiver
    environment:
      - COLLECTOR_OTLP_ENABLED=true

  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command: ["--config=/etc/otel-collector-config.yaml"]
    volumes:
      - ./otel-collector-config.yaml:/etc/otel-collector-config.yaml
    ports:
      - "4318:4318"  # OTLP HTTP receiver
      - "4317:4317"  # OTLP gRPC receiver
    depends_on:
      - jaeger
```

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [jaeger]
```

Start the stack:

```bash
docker-compose up -d

# Configure Grid
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_INSECURE=true

# Start Grid API
./bin/gridapi serve
```

## Trace Examples

### Example 1: Create State Request

```
HTTP Request: POST /api/states
  └─ state.CreateState
      ├─ (attributes: state.guid, state.logic_id, state.label_count)
      ├─ (event: validation.failed) [if labels invalid]
      └─ SQL: INSERT INTO states ...
          └─ (span created by bunotel)
```

### Example 2: Authenticate + List States

```
HTTP Request: GET /api/states (with Authorization header)
  ├─ iam.AuthenticateRequest
  │   ├─ (event: authentication.succeeded)
  │   └─ iam.ResolveRoles
  │       ├─ (attributes: principal.id, group_count)
  │       ├─ SQL: SELECT * FROM user_roles ...
  │       └─ SQL: SELECT * FROM roles ...
  └─ state.ListStates
      ├─ SQL: SELECT * FROM states ...
      └─ (attributes: state.count)
```

## Span Attributes

Grid uses semantic conventions for trace attributes:

### State Service
- `state.guid`: State GUID
- `state.logic_id`: State logic ID
- `state.label_count`: Number of labels
- `state.count`: Number of states returned

### IAM Service
- `principal.id`: Principal identifier
- `principal.type`: Principal type (user/service_account)
- `principal.role`: Principal's role
- `authenticator_count`: Number of authenticators
- `authenticator_index`: Which authenticator succeeded
- `is_user`: Whether principal is a user (vs service account)
- `group_count`: Number of groups
- `direct_role_count`: Number of directly assigned roles

### Dependency Service
- `dependency.source_guid`: Source state GUID
- `dependency.target_guid`: Target state GUID
- `dependency.edge_type`: Type of dependency edge

### Policy Service
- `policy.action`: Action being authorized
- `policy.resource`: Resource being accessed
- `policy.allowed`: Authorization result

## Span Events

Grid records business events during trace execution:

### Authentication Events
- `authentication.succeeded`: Successful authentication
  - Attributes: `principal_id`, `authenticator_index`
- `authentication.failed`: Failed authentication
  - Attributes: `authenticator_index`, `error`
- `authentication.no_credentials`: No credentials provided

### Validation Events
- `validation.failed`: Validation failure
  - Attributes: `reason`
- `policy.check`: Policy authorization check
  - Attributes: `action`, `allowed`

## Performance Considerations

### Zero Overhead When Disabled
- If `OTEL_EXPORTER_OTLP_ENDPOINT` is not set, Grid uses noop providers
- No performance impact whatsoever
- Instrumentation code remains in place but does nothing

### Minimal Overhead When Enabled
- **Bun OTEL**: Only creates spans when active trace context exists
- **Chi Middleware**: Uses batch span processor (async export)
- **Service Layer**: Span creation is <1μs
- **Expected overhead**: <5ms per request (network export is async)

### Optimization Tips
1. Use batch span processor (default) instead of simple span processor
2. Configure sampling in OTEL collector (not in Grid)
3. Use local collector with buffering, not direct export to backend
4. Monitor collector health and backpressure

## Troubleshooting

### No traces appearing in backend

1. **Check Grid logs**: Look for "Initializing OpenTelemetry" message
   ```
   Initializing OpenTelemetry: endpoint=http://localhost:4318, protocol=http/protobuf, service=gridapi
   ```

2. **Verify endpoint is reachable**:
   ```bash
   curl -v http://localhost:4318/v1/traces
   # Should return 405 Method Not Allowed (POST expected)
   ```

3. **Check collector logs**:
   ```bash
   docker logs otel-collector
   # Look for OTLP receiver errors
   ```

4. **Test with dummy trace**:
   ```bash
   curl -X POST http://localhost:4318/v1/traces \
     -H "Content-Type: application/json" \
     -d '{"resourceSpans":[]}'
   # Should return 200 OK
   ```

### Traces incomplete or missing spans

1. **Verify context propagation**: Ensure W3C Trace Context headers are present
   ```bash
   curl -v http://localhost:8080/api/states \
     -H "traceparent: 00-$(openssl rand -hex 16)-$(openssl rand -hex 8)-01"
   ```

2. **Check service instrumentation**: Look for span creation in service code
3. **Verify Bun hook is installed**: Check for `db.AddQueryHook(bunotel.NewQueryHook(...))`

### High latency impact

1. **Use async export**: Verify batch span processor is configured (default)
2. **Check collector backpressure**: Monitor OTLP receiver queue depth
3. **Reduce sampling rate**: Configure sampler in collector config
4. **Use local collector**: Don't export directly to remote backend

## Production Deployment

### Recommended Setup

1. **Run OTEL Collector as sidecar or DaemonSet**
   - Grid exports to local collector (low latency)
   - Collector batches and exports to backend (Jaeger, Tempo, etc.)
   - Collector handles retries, backpressure, sampling

2. **Use TLS for production**
   ```bash
   export OTEL_EXPORTER_OTLP_ENDPOINT=https://collector.example.com:4318
   export OTEL_EXPORTER_OTLP_INSECURE=false  # Default
   ```

3. **Set service metadata**
   ```bash
   export OTEL_SERVICE_NAME=gridapi
   export OTEL_SERVICE_VERSION=$(git describe --tags)
   export OTEL_DEPLOYMENT_ENVIRONMENT=production
   ```

4. **Configure sampling** (in collector, not Grid)
   ```yaml
   # collector-config.yaml
   processors:
     probabilistic_sampler:
       sampling_percentage: 10  # Sample 10% of traces
   ```

## Further Reading

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/)
- [Bun Performance Monitoring](https://bun.uptrace.dev/guide/performance-monitoring.html)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/latest/)
- [Grid Observability Design](../specs/observability-otel-design.md)

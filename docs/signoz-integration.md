# SigNoz Integration Guide

This guide explains how to integrate Grid API with SigNoz for comprehensive observability (logs, traces, and metrics).

## What is SigNoz?

SigNoz is an open-source observability platform that provides:
- **Distributed Tracing**: Visualize request flows across services
- **Metrics Monitoring**: Time-series metrics with dashboards and alerts
- **Log Management**: Structured logs with powerful querying and correlation
- **Service Maps**: Automatic service topology visualization
- **Alerting**: Flexible alerting on metrics, logs, and traces

**Why SigNoz?**
- **All-in-One**: Logs, traces, and metrics in single platform (vs separate tools)
- **Cost-Effective**: 50% lower resource usage vs Elasticsearch/Splunk
- **Open Source**: Self-hosted, no vendor lock-in
- **Native OTLP**: First-class OpenTelemetry support
- **ClickHouse Backend**: High-performance columnar database for analytics

## Quick Start

### 1. Start SigNoz Stack

```bash
# From Grid repository root
cd /home/user/grid

# Start SigNoz (takes ~2 minutes on first run)
docker compose -f docker-compose.signoz.yml up -d

# Check status
docker compose -f docker-compose.signoz.yml ps

# Expected output:
# signoz-clickhouse       Running
# signoz-query-service    Running
# signoz-otel-collector   Running
# signoz-frontend         Running
# signoz-alertmanager     Running
# signoz-zookeeper        Running

# Access UI
open http://localhost:3301
```

**First-time Setup:**
1. Create admin account (email + password)
2. Skip the "Install OTEL" tutorial (Grid already has OTEL integrated)
3. You'll see "Waiting for data..." until Grid sends telemetry

### 2. Configure Grid API

```bash
# Set OTLP endpoint (SigNoz collector)
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_INSECURE=true  # Required for local development

# Optional: Service metadata
export OTEL_SERVICE_NAME=gridapi
export OTEL_SERVICE_VERSION=1.0.0
export OTEL_DEPLOYMENT_ENVIRONMENT=local

# Start Grid API
./bin/gridapi serve
```

**Expected logs:**
```
Initializing OpenTelemetry: endpoint=localhost:4317, protocol=grpc, service=gridapi
WARNING: Using insecure gRPC connection (development only)
OpenTelemetry initialized successfully (traces, metrics, logs)
```

### 3. Generate Telemetry Data

```bash
# Make some API calls to generate telemetry
curl -X POST http://localhost:8080/api/states \
  -H "Content-Type: application/json" \
  -d '{"guid":"01234567-89ab-cdef-0123-456789abcdef","logic_id":"test-state-1"}'

curl http://localhost:8080/api/states

curl http://localhost:8080/health
```

### 4. View in SigNoz

Navigate to http://localhost:3301 and explore:

**Services Tab:**
- See "gridapi" service appear (refresh if needed)
- View request rate, error rate, latency (P50/P90/P99)
- Click service name → View service details

**Traces Tab:**
- See individual HTTP requests as traces
- Click a trace → View span hierarchy (HTTP → Service → Database)
- See span attributes (state.guid, principal.id, etc.)
- See SQL queries with formatted SQL

**Logs Tab (Future):**
- Structured logs with trace correlation
- Filter by trace_id, span_id, severity
- Full-text search across log messages

**Metrics Tab:**
- HTTP request counts by route/status
- Request latency histograms
- Active connection counts
- Database query metrics

## Configuration

### Protocol Selection: gRPC vs HTTP

**gRPC (Recommended):**
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317  # Port 4317 for gRPC
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

**Advantages:**
- Lower overhead (binary protocol, HTTP/2)
- Better performance (multiplexing, header compression)
- Recommended by OpenTelemetry and SigNoz

**HTTP/Protobuf (Alternative):**
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318  # Port 4318 for HTTP
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

**Advantages:**
- Easier debugging (use curl to inspect traffic)
- More compatible with firewalls/proxies
- Simpler troubleshooting

### Environment Variables Reference

```bash
# === Required for Enablement ===
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317  # SigNoz OTLP collector endpoint

# === Protocol Configuration ===
OTEL_EXPORTER_OTLP_PROTOCOL=grpc            # grpc (recommended) or http/protobuf
OTEL_EXPORTER_OTLP_INSECURE=true            # Use insecure connection (dev only)

# === Service Identification ===
OTEL_SERVICE_NAME=gridapi                    # Service name in SigNoz
OTEL_SERVICE_VERSION=1.0.0                   # Service version (from build)
OTEL_DEPLOYMENT_ENVIRONMENT=production       # Environment tag

# === Advanced Options ===
OTEL_RESOURCE_ATTRIBUTES=key1=val1,key2=val2 # Custom resource attributes
OTEL_EXPORTER_OTLP_HEADERS=key=value         # Custom headers (for auth)
OTEL_EXPORTER_OTLP_TIMEOUT=10000             # Export timeout (ms)
```

### Production Configuration

For production deployment with SigNoz Cloud or self-hosted with TLS:

```bash
# SigNoz Cloud
export OTEL_EXPORTER_OTLP_ENDPOINT=ingest.{region}.signoz.cloud:443
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_INSECURE=false  # Use TLS
export OTEL_EXPORTER_OTLP_HEADERS=signoz-access-token=<YOUR_TOKEN>

# Self-hosted SigNoz with TLS
export OTEL_EXPORTER_OTLP_ENDPOINT=signoz.example.com:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_INSECURE=false
```

## Telemetry Types

### 1. Traces (Distributed Tracing)

**What Grid Sends:**
- HTTP request spans (method, route, status, latency)
- Service operation spans (state.CreateState, iam.Authenticate, etc.)
- Database query spans (SQL, duration, affected rows)

**Example Trace:**
```
POST /api/states [200 OK, 45ms]
├─ state.CreateState [42ms]
│  ├─ validation.labels [2ms]
│  └─ SQL: INSERT INTO states ... [8ms]
└─ (span attributes: state.guid, state.logic_id)
```

**How to View:**
1. Navigate to **Traces** tab in SigNoz
2. Filter by service: `gridapi`
3. Click a trace → View flame graph
4. Click spans → View attributes, events, errors

**Use Cases:**
- Debug slow requests (identify bottlenecks)
- Investigate errors (see full context)
- Understand request flow across services

### 2. Metrics (Time-Series Data)

**What Grid Sends:**
- HTTP request count (by method, route, status)
- HTTP request duration (histogram for percentiles)
- HTTP active connections (current load)
- HTTP error count (5xx responses)
- Database query count/duration
- Authentication attempts/failures

**Example Metrics:**
```
http.server.request.count{method="POST",route="/api/states",status="200"} = 1543
http.server.request.duration{method="POST",route="/api/states",p99} = 125ms
http.server.active_connections = 12
http.server.error.count{method="POST",route="/api/states"} = 3
```

**How to View:**
1. Navigate to **Dashboards** tab in SigNoz
2. Create custom dashboard or use built-in APM dashboard
3. Add panels for key metrics
4. Use PromQL or visual query builder

**Use Cases:**
- Monitor service health (request rate, error rate, latency)
- Capacity planning (active connections, resource usage)
- SLI/SLO tracking (uptime, latency percentiles)

### 3. Logs (Structured Logging)

**Status:** Coming in future update

**Planned Features:**
- Structured logs with JSON format
- Automatic trace correlation (trace_id, span_id in logs)
- Log levels (DEBUG, INFO, WARN, ERROR)
- Contextual attributes (user_id, request_id, etc.)

**Example Log:**
```json
{
  "timestamp": "2025-12-01T10:30:45.123Z",
  "level": "ERROR",
  "message": "Failed to create state",
  "trace_id": "3d2f8a6b9c7e1234567890abcdef1234",
  "span_id": "7890abcdef123456",
  "attributes": {
    "state.guid": "01234567-89ab-cdef-0123-456789abcdef",
    "error.type": "ValidationError",
    "user_id": "user-123"
  }
}
```

## SigNoz Features

### Service Map

**What it shows:**
- Visual graph of service dependencies
- Request rate between services (requests/sec)
- Error rate for each service
- Latency (P99) between services

**How to access:**
1. Navigate to **Services** tab
2. Select `gridapi` service
3. Click **Service Map** tab
4. Explore dependencies and metrics overlay

**Use cases:**
- Understand system architecture at a glance
- Identify high-latency dependencies
- Spot cascading failures
- Plan service migrations

### Log Queries

**Query Language:**
SigNoz uses a SQL-like syntax for log queries:

```sql
-- Find all errors in last hour
SELECT * FROM logs
WHERE severity_text = 'ERROR'
  AND timestamp > now() - INTERVAL 1 HOUR
ORDER BY timestamp DESC

-- Find logs for specific trace
SELECT * FROM logs
WHERE trace_id = '3d2f8a6b9c7e1234567890abcdef1234'
ORDER BY timestamp ASC

-- Find logs with specific attribute
SELECT * FROM logs
WHERE body.user_id = '123'
  AND attributes.endpoint = '/api/states'

-- Aggregate error counts by service
SELECT
  count(*) as error_count,
  attributes.service_name
FROM logs
WHERE severity_text = 'ERROR'
GROUP BY attributes.service_name
ORDER BY error_count DESC
```

**Query Builder:**
- Visual query builder for no-code queries
- Auto-complete for fields and values
- One-click dashboard/alert creation

### Dashboards

**Pre-built Dashboards:**
- **APM Dashboard**: Request rate, error rate, latency
- **Infrastructure**: CPU, memory, disk usage
- **Database**: Query performance, connection pools

**Custom Dashboards:**
1. Navigate to **Dashboards** → **New Dashboard**
2. Add panels (Time Series, Gauge, Table, etc.)
3. Configure queries (PromQL or ClickHouse SQL)
4. Set visualization options
5. Save and share dashboard

**Example Panel:**
```
Panel: HTTP Request Rate
Query: sum(rate(http_server_request_count[5m])) by (method, route)
Visualization: Stacked Area Chart
Legend: {{method}} {{route}}
```

### Alerts

**Alert Types:**

**1. Metric-based Alerts:**
```yaml
alert_name: "High Error Rate"
condition: |
  sum(rate(http_server_error_count[5m])) > 10
evaluation_interval: 1m
notification: slack, pagerduty
```

**2. Log-based Alerts:**
```yaml
alert_name: "Authentication Failures Spike"
condition: |
  count(severity_text='ERROR' AND message LIKE '%auth%') > 100
time_window: 5m
notification: email
```

**3. Trace-based Alerts:**
```yaml
alert_name: "Slow Database Queries"
condition: |
  p99(db.query.duration) > 1000ms
evaluation_interval: 5m
notification: slack
```

**How to Create:**
1. Navigate to **Alerts** → **New Alert**
2. Select alert type (metric/log/trace)
3. Configure query and threshold
4. Set evaluation window
5. Add notification channels
6. Save alert

## Troubleshooting

### No Data Appearing in SigNoz

**Check 1: Verify Grid OTEL initialization**
```bash
# Check Grid API logs for:
"Initializing OpenTelemetry: endpoint=localhost:4317, protocol=grpc, service=gridapi"
"OpenTelemetry initialized successfully (traces, metrics, logs)"

# If missing, verify environment variables:
env | grep OTEL
```

**Check 2: Verify SigNoz collector is running**
```bash
docker compose -f docker-compose.signoz.yml ps signoz-otel-collector

# Expected: Running, healthy
# If unhealthy, check logs:
docker compose -f docker-compose.signoz.yml logs signoz-otel-collector
```

**Check 3: Test collector endpoint**
```bash
# For gRPC (harder to test manually, use grpcurl)
grpcurl -plaintext localhost:4317 list

# For HTTP (easier to test)
curl -v http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{"resourceSpans":[]}'

# Expected: 200 OK (even with empty data)
```

**Check 4: Generate telemetry and wait**
```bash
# Make several API calls
for i in {1..10}; do
  curl http://localhost:8080/health
  sleep 1
done

# Wait 30 seconds (for batch export)
# Refresh SigNoz UI
```

### Traces Missing Database Spans

**Cause:** Bun OTEL hook not active or context not propagated

**Fix:**
1. Verify Bun hook is installed (in serve.go):
   ```go
   db.AddQueryHook(bunotel.NewQueryHook(
       bunotel.WithDBName("grid"),
       bunotel.WithFormattedQueries(true),
   ))
   ```

2. Verify context is passed to queries:
   ```go
   // ❌ Wrong: no context
   err := db.NewSelect().Model(&states).Scan()

   // ✅ Correct: pass context from request
   err := db.NewSelect().Model(&states).Scan(ctx)
   ```

### High Latency Impact

**Symptoms:**
- Requests slower with OTEL enabled
- CPU usage increased

**Diagnosis:**
```bash
# Check batch processor is used (not simple processor)
# In telemetry/otel.go, look for:
sdktrace.WithBatcher(traceExporter, ...)  # ✅ Good
# NOT:
sdktrace.WithSyncer(traceExporter)  # ❌ Bad (synchronous export)

# Check export interval is reasonable
log.WithExportInterval(5*time.Second)  # ✅ Good (5-10s)
log.WithExportInterval(100*time.Millisecond)  # ❌ Bad (too frequent)
```

**Fixes:**
1. Use batch processors (default in Grid)
2. Increase batch timeout (5-10s)
3. Use gRPC instead of HTTP (lower overhead)
4. Enable sampling in production (not AlwaysSample)

### SigNoz UI Slow or Unresponsive

**Cause:** ClickHouse under load or insufficient resources

**Diagnosis:**
```bash
# Check ClickHouse resource usage
docker stats signoz-clickhouse

# Expected: <2GB memory, <50% CPU
# If higher, increase Docker memory limit
```

**Fixes:**
1. Increase Docker memory allocation (4GB → 8GB)
2. Reduce data retention (default: 30 days)
3. Add sampling to reduce data volume
4. Scale ClickHouse vertically (more CPU/memory)

## Best Practices

### 1. Resource Attributes

Always set these resource attributes:

```bash
export OTEL_SERVICE_NAME=gridapi              # Required
export OTEL_SERVICE_VERSION=1.0.0             # Highly recommended
export OTEL_DEPLOYMENT_ENVIRONMENT=production # Recommended
```

**Why?**
- `service.name`: Groups telemetry by service (critical for multi-service systems)
- `service.version`: Track performance across deployments
- `deployment.environment`: Separate prod/staging/dev data

### 2. Metric Cardinality

**❌ Avoid high-cardinality attributes:**
```go
// Bad: Millions of unique values
metric.WithAttributes(
    attribute.String("user_id", userID),     // ❌ High cardinality
    attribute.String("request_id", reqID),   // ❌ High cardinality
)
```

**✅ Use low-cardinality attributes:**
```go
// Good: Bounded dimensions
metric.WithAttributes(
    attribute.String("http.method", "POST"), // ✅ ~10 values
    attribute.String("http.route", "/api/states"), // ✅ ~50 values
    attribute.String("http.status_code", "200"), // ✅ ~10 values
)
```

**Why?**
- High cardinality → millions of time series → slow queries, high memory usage
- Low cardinality → manageable data, fast queries, lower costs

### 3. Trace Sampling

**Development:**
```go
sdktrace.WithSampler(sdktrace.AlwaysSample())  // Trace everything
```

**Production:**
```go
// Trace based on parent decision (propagated from upstream)
sdktrace.WithSampler(sdktrace.ParentBased(
    sdktrace.TraceIDRatioBased(0.1),  // Sample 10% of traces
))
```

**Why?**
- AlwaysSample in prod → massive data volume, high costs
- ParentBased + ratio → consistent traces (all spans or none)

### 4. Graceful Shutdown

Always flush telemetry on shutdown:

```go
defer func() {
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := shutdownTelemetry(shutdownCtx); err != nil {
        log.Printf("ERROR: Failed to shutdown telemetry: %v", err)
    }
}()
```

**Why?**
- Pending telemetry in batches is lost without shutdown
- Graceful shutdown ensures all data is exported

## Next Steps

### Phase 1: Traces ✅ (Complete)
- HTTP tracing via otelchi middleware
- Service-layer tracing (state, IAM)
- Database tracing via Bun hook

### Phase 2: Metrics (In Progress)
- [x] Metrics infrastructure (MeterProvider)
- [ ] HTTP metrics middleware
- [ ] Database metrics via Bun hook
- [ ] Custom business metrics

### Phase 3: Logs (Future)
- [ ] Structured logging with slog/logrus
- [ ] OTEL log exporter integration
- [ ] Trace correlation (trace_id, span_id in logs)
- [ ] Log aggregation in SigNoz

### Phase 4: Advanced (Future)
- [ ] Custom dashboards for Grid KPIs
- [ ] Alerting rules (error rate, latency, etc.)
- [ ] Service SLIs/SLOs
- [ ] Anomaly detection

## Resources

**Official Documentation:**
- [SigNoz Documentation](https://signoz.io/docs/)
- [SigNoz Docker Setup](https://signoz.io/docs/install/docker/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/)

**Grid Documentation:**
- [Observability Overview](observability.md)
- [OTEL Design Document](../specs/observability-otel-design.md)
- [Dependency Installation](otel-dependencies.md)

**Community:**
- [SigNoz Slack](https://signoz.io/slack)
- [SigNoz GitHub](https://github.com/SigNoz/signoz)
- [OpenTelemetry Community](https://cloud-native.slack.com/) (#otel)

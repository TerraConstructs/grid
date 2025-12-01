package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ServerMetrics holds metric instruments for HTTP server telemetry.
// Initialize once at server startup and reuse throughout the application lifecycle.
type ServerMetrics struct {
	RequestCounter    metric.Int64Counter      // Total HTTP requests
	RequestDuration   metric.Float64Histogram  // HTTP request latency
	ActiveConnections metric.Int64UpDownCounter // Active HTTP connections
	ErrorCounter      metric.Int64Counter      // Total HTTP errors (5xx)
}

// NewServerMetrics creates a new ServerMetrics instance with pre-configured instruments.
// Call this during server initialization and store the returned metrics globally.
func NewServerMetrics() (*ServerMetrics, error) {
	meter := otel.Meter("gridapi/http")

	// Counter: Total number of HTTP requests
	// Use for: Request counts by method, route, status
	requestCounter, err := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	// Histogram: HTTP request duration in milliseconds
	// Use for: Latency percentiles (p50, p95, p99)
	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("ms"),
		// Buckets: 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s
		metric.WithExplicitBucketBoundaries(5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000),
	)
	if err != nil {
		return nil, err
	}

	// UpDownCounter: Number of active HTTP connections
	// Use for: Current load, connection pool monitoring
	activeConnections, err := meter.Int64UpDownCounter(
		"http.server.active_connections",
		metric.WithDescription("Number of active HTTP connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, err
	}

	// Counter: Total number of HTTP errors (5xx responses)
	// Use for: Error rate alerts, SLI calculations
	errorCounter, err := meter.Int64Counter(
		"http.server.error.count",
		metric.WithDescription("Total number of HTTP server errors (5xx)"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	return &ServerMetrics{
		RequestCounter:    requestCounter,
		RequestDuration:   requestDuration,
		ActiveConnections: activeConnections,
		ErrorCounter:      errorCounter,
	}, nil
}

// RecordRequest records an HTTP request with method, route, status, and duration.
// Call this at the end of each request handler (typically in middleware).
func (m *ServerMetrics) RecordRequest(ctx context.Context, method, route, status string, durationMs float64) {
	// Attributes for dimensions (allows filtering/grouping in SigNoz)
	attrs := metric.WithAttributes(
		attribute.String("http.method", method),
		attribute.String("http.route", route),
		attribute.String("http.status_code", status),
	)

	// Increment request counter
	m.RequestCounter.Add(ctx, 1, attrs)

	// Record request duration
	m.RequestDuration.Record(ctx, durationMs, attrs)

	// Increment error counter if 5xx status
	if len(status) > 0 && status[0] == '5' {
		m.ErrorCounter.Add(ctx, 1, attrs)
	}
}

// ConnectionOpened increments the active connections counter.
// Call this when a new HTTP connection is established.
func (m *ServerMetrics) ConnectionOpened(ctx context.Context) {
	m.ActiveConnections.Add(ctx, 1)
}

// ConnectionClosed decrements the active connections counter.
// Call this when an HTTP connection is closed.
func (m *ServerMetrics) ConnectionClosed(ctx context.Context) {
	m.ActiveConnections.Add(ctx, -1)
}

// DatabaseMetrics holds metric instruments for database operations.
type DatabaseMetrics struct {
	QueryCounter  metric.Int64Counter     // Total database queries
	QueryDuration metric.Float64Histogram // Query latency
	QueryErrors   metric.Int64Counter     // Total query errors
}

// NewDatabaseMetrics creates metric instruments for database telemetry.
func NewDatabaseMetrics() (*DatabaseMetrics, error) {
	meter := otel.Meter("gridapi/database")

	queryCounter, err := meter.Int64Counter(
		"db.query.count",
		metric.WithDescription("Total number of database queries"),
		metric.WithUnit("{query}"),
	)
	if err != nil {
		return nil, err
	}

	queryDuration, err := meter.Float64Histogram(
		"db.query.duration",
		metric.WithDescription("Database query duration"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 250, 500, 1000, 2000),
	)
	if err != nil {
		return nil, err
	}

	queryErrors, err := meter.Int64Counter(
		"db.query.error.count",
		metric.WithDescription("Total number of database query errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	return &DatabaseMetrics{
		QueryCounter:  queryCounter,
		QueryDuration: queryDuration,
		QueryErrors:   queryErrors,
	}, nil
}

// RecordQuery records a database query with operation type and duration.
func (d *DatabaseMetrics) RecordQuery(ctx context.Context, operation string, durationMs float64, err error) {
	attrs := metric.WithAttributes(
		attribute.String("db.operation", operation), // SELECT, INSERT, UPDATE, DELETE
	)

	d.QueryCounter.Add(ctx, 1, attrs)
	d.QueryDuration.Record(ctx, durationMs, attrs)

	if err != nil {
		d.QueryErrors.Add(ctx, 1, attrs)
	}
}

// AuthMetrics holds metric instruments for authentication operations.
type AuthMetrics struct {
	AuthAttempts metric.Int64Counter // Total auth attempts
	AuthFailures metric.Int64Counter // Failed auth attempts
	AuthDuration metric.Float64Histogram
}

// NewAuthMetrics creates metric instruments for authentication telemetry.
func NewAuthMetrics() (*AuthMetrics, error) {
	meter := otel.Meter("gridapi/auth")

	authAttempts, err := meter.Int64Counter(
		"auth.attempt.count",
		metric.WithDescription("Total number of authentication attempts"),
		metric.WithUnit("{attempt}"),
	)
	if err != nil {
		return nil, err
	}

	authFailures, err := meter.Int64Counter(
		"auth.failure.count",
		metric.WithDescription("Total number of failed authentication attempts"),
		metric.WithUnit("{failure}"),
	)
	if err != nil {
		return nil, err
	}

	authDuration, err := meter.Float64Histogram(
		"auth.duration",
		metric.WithDescription("Authentication operation duration"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(5, 10, 25, 50, 100, 250, 500, 1000),
	)
	if err != nil {
		return nil, err
	}

	return &AuthMetrics{
		AuthAttempts: authAttempts,
		AuthFailures: authFailures,
		AuthDuration: authDuration,
	}, nil
}

// RecordAuth records an authentication attempt with result and duration.
func (a *AuthMetrics) RecordAuth(ctx context.Context, method string, success bool, durationMs float64) {
	attrs := metric.WithAttributes(
		attribute.String("auth.method", method), // jwt, session, etc.
		attribute.Bool("auth.success", success),
	)

	a.AuthAttempts.Add(ctx, 1, attrs)
	a.AuthDuration.Record(ctx, durationMs, attrs)

	if !success {
		a.AuthFailures.Add(ctx, 1, attrs)
	}
}

// Common metric attribute keys for Grid services
const (
	// HTTP attributes
	AttrHTTPMethod     = "http.method"
	AttrHTTPRoute      = "http.route"
	AttrHTTPStatusCode = "http.status_code"

	// Database attributes
	AttrDBOperation = "db.operation"
	AttrDBTable     = "db.table"

	// Auth attributes
	AttrAuthMethod  = "auth.method"
	AttrAuthSuccess = "auth.success"

	// State service attributes
	AttrStateOperation = "state.operation" // create, list, get, update, delete
)

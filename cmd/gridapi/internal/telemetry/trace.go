package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan creates a new span for a service operation.
// This is a convenience wrapper around otel.Tracer().Start() with common patterns.
//
// Usage in services:
//
//	ctx, span := telemetry.StartSpan(ctx, "gridapi/services/state", "state.CreateState",
//	    attribute.String("state.guid", guid),
//	    attribute.String("state.logic_id", logicID),
//	)
//	defer span.End()
func StartSpan(ctx context.Context, tracerName, spanName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	return tracer.Start(ctx, spanName, trace.WithAttributes(attrs...))
}

// RecordError records an error on the span and sets the span status to error.
// This is a convenience wrapper to ensure consistent error recording.
func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// AddEvent adds a named event to the span with optional attributes.
// Use for business events like validation failures, policy checks, etc.
//
// Example:
//
//	telemetry.AddEvent(span, "validation.failed",
//	    attribute.String("reason", "invalid label format"),
//	)
func AddEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// Common attribute keys for Grid services
const (
	// State service attributes
	AttrStateGUID    = "state.guid"
	AttrStateLogicID = "state.logic_id"
	AttrStateLabels  = "state.labels"

	// Dependency service attributes
	AttrDependencySourceGUID = "dependency.source_guid"
	AttrDependencyTargetGUID = "dependency.target_guid"
	AttrDependencyEdgeType   = "dependency.edge_type"

	// IAM service attributes
	AttrPrincipalID   = "principal.id"
	AttrPrincipalType = "principal.type"
	AttrPrincipalRole = "principal.role"

	// Policy service attributes
	AttrPolicyAction   = "policy.action"
	AttrPolicyResource = "policy.resource"
	AttrPolicyAllowed  = "policy.allowed"

	// Output service attributes
	AttrOutputKey    = "output.key"
	AttrOutputSchema = "output.schema"
)

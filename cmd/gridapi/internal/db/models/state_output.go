package models

import (
	"time"

	"github.com/uptrace/bun"
)

// StateOutput represents a cached Terraform/OpenTofu output key from a state's JSON.
// This table enables fast cross-state output searches without parsing every state's JSON.
// It also stores optional JSON Schema definitions for outputs, allowing clients to declare
// expected output types before the output actually exists in the Terraform state.
type StateOutput struct {
	bun.BaseModel `bun:"table:state_outputs,alias:so"`

	StateGUID   string `bun:"state_guid,pk,type:uuid,notnull"`
	OutputKey   string `bun:"output_key,pk,type:text,notnull"`
	Sensitive   bool   `bun:"sensitive,notnull,default:false"`
	StateSerial int64  `bun:"state_serial,notnull"`

	// SchemaJSON stores an optional JSON Schema definition for this output.
	// This allows clients to publish type information before the output exists in state.
	// Stored as TEXT to allow arbitrary JSON Schema complexity.
	SchemaJSON *string `bun:"schema_json,type:text,nullzero"`

	// SchemaSource indicates whether the schema was manually set or automatically inferred.
	// Values: "manual" (set via SetOutputSchema), "inferred" (auto-generated from output value)
	// NULL when no schema exists.
	SchemaSource *string `bun:"schema_source,type:text,nullzero"`

	// ValidationStatus indicates the result of JSON Schema validation.
	// Values: "valid" (passed validation), "invalid" (failed validation), "error" (validation error)
	// NULL when no schema exists or validation hasn't run.
	ValidationStatus *string `bun:"validation_status,type:text,nullzero"`

	// ValidationError contains the validation error message with JSON path information.
	// NULL when validation_status is "valid" or hasn't run.
	ValidationError *string `bun:"validation_error,type:text,nullzero"`

	// ValidatedAt is the timestamp of the last validation run.
	// NULL when validation hasn't run.
	ValidatedAt *time.Time `bun:"validated_at,type:timestamptz,nullzero"`

	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull,default:current_timestamp"`

	// Relationships for eager loading (populated only when using Relation())
	State *State `bun:"rel:belongs-to,join:state_guid=guid"`
}

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

	StateGUID   string    `bun:"state_guid,pk,type:uuid,notnull"`
	OutputKey   string    `bun:"output_key,pk,type:text,notnull"`
	Sensitive   bool      `bun:"sensitive,notnull,default:false"`
	StateSerial int64     `bun:"state_serial,notnull"`

	// SchemaJSON stores an optional JSON Schema definition for this output.
	// This allows clients to publish type information before the output exists in state.
	// Stored as TEXT to allow arbitrary JSON Schema complexity.
	SchemaJSON  *string   `bun:"schema_json,type:text,nullzero"`

	CreatedAt   time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt   time.Time `bun:"updated_at,notnull,default:current_timestamp"`

	// Relationships for eager loading (populated only when using Relation())
	State *State `bun:"rel:belongs-to,join:state_guid=guid"`
}

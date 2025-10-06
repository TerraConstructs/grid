package models

import (
	"time"

	"github.com/uptrace/bun"
)

// StateOutput represents a cached Terraform/OpenTofu output key from a state's JSON.
// This table enables fast cross-state output searches without parsing every state's JSON.
type StateOutput struct {
	bun.BaseModel `bun:"table:state_outputs,alias:so"`

	StateGUID   string    `bun:"state_guid,pk,type:uuid,notnull"`
	OutputKey   string    `bun:"output_key,pk,type:text,notnull"`
	Sensitive   bool      `bun:"sensitive,notnull,default:false"`
	StateSerial int64     `bun:"state_serial,notnull"`
	CreatedAt   time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt   time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

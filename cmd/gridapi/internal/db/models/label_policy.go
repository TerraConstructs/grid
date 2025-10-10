package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// PolicyDefinition represents label validation rules
type PolicyDefinition struct {
	AllowedKeys      map[string]struct{} `json:"allowed_keys,omitempty"`
	AllowedValues    map[string][]string `json:"allowed_values,omitempty"`
	ReservedPrefixes []string            `json:"reserved_prefixes,omitempty"`
	MaxKeys          int                 `json:"max_keys"`
	MaxValueLen      int                 `json:"max_value_len"`
}

// Scan implements sql.Scanner
func (pd *PolicyDefinition) Scan(value any) error {
	if value == nil {
		*pd = PolicyDefinition{}
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("failed to scan PolicyDefinition: expected []byte or string, got %T", value)
	}

	if err := json.Unmarshal(raw, pd); err != nil {
		return fmt.Errorf("failed to unmarshal PolicyDefinition: %w", err)
	}

	// Apply defaults for zero values
	if pd.MaxKeys == 0 {
		pd.MaxKeys = 32
	}
	if pd.MaxValueLen == 0 {
		pd.MaxValueLen = 256
	}

	return nil
}

// Value implements driver.Valuer
func (pd PolicyDefinition) Value() (driver.Value, error) {
	bytes, err := json.Marshal(pd)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

// LabelPolicy represents the single-row label policy table
type LabelPolicy struct {
	bun.BaseModel `bun:"table:label_policy,alias:lp"`

	ID         int              `bun:"id,pk"`
	Version    int              `bun:"version,notnull"`
	PolicyJSON PolicyDefinition `bun:"policy_json,type:jsonb,notnull"`
	CreatedAt  time.Time        `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt  time.Time        `bun:"updated_at,notnull,default:current_timestamp"`
}

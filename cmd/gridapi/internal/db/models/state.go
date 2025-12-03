package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const StateSizeWarningThreshold = 10 * 1024 * 1024 // 10MB

// LabelMap represents typed label values (string | float64 | bool)
type LabelMap map[string]any

// Scan implements sql.Scanner for reading from database
func (lm *LabelMap) Scan(value any) error {
	if value == nil {
		*lm = make(LabelMap)
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to scan LabelMap: expected []byte or string, got %T", value)
	}

	return json.Unmarshal(bytes, lm)
}

// Value implements driver.Valuer for writing to database
func (lm LabelMap) Value() (driver.Value, error) {
	if lm == nil {
		return "{}", nil
	}
	bytes, err := json.Marshal(lm)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

// State represents the persisted Terraform state
type State struct {
	bun.BaseModel `bun:"table:states,alias:s"`

	GUID         string    `bun:"guid,pk,type:uuid"`
	LogicID      string    `bun:"logic_id,notnull,unique"`
	StateContent []byte    `bun:"state_content,type:bytea"`
	SizeBytes    int64     `bun:"size_bytes,scanonly"`
	Locked       bool      `bun:"locked,notnull,default:false"`
	LockInfo     *LockInfo `bun:"lock_info,type:jsonb"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"updated_at,notnull,default:current_timestamp"`

	// Labels stores typed label key/value pairs
	Labels LabelMap `bun:"labels,type:jsonb,notnull,default:'{}'"`

	// Relationships for eager loading (populated only when using Relation())
	Outputs       []*StateOutput `bun:"rel:has-many,join:guid=state_guid"`
	OutgoingEdges []*Edge        `bun:"rel:has-many,join:guid=from_state"`
	IncomingEdges []*Edge        `bun:"rel:has-many,join:guid=to_state"`

	// Computed counts (populated via subqueries for efficient StateInfo rendering)
	// These are scanonly fields populated by COUNT subqueries in SELECT statements
	DependenciesCount int `bun:"dependencies_count,scanonly"`
	DependentsCount   int `bun:"dependents_count,scanonly"`
	OutputsCount      int `bun:"outputs_count,scanonly"`
}

// LockInfo captures Terraform lock metadata stored with the state.
type LockInfo struct {
	ID               string    `json:"ID"`
	Operation        string    `json:"Operation"`
	Info             string    `json:"Info"`
	Who              string    `json:"Who"`
	Version          string    `json:"Version"`
	Created          time.Time `json:"Created"`
	Path             string    `json:"Path"`
	OwnerPrincipalID string    `json:"owner_principal_id,omitempty"`
}

// ValidateForCreate verifies the record is well formed before insertion.
func (s *State) ValidateForCreate() error {
	if _, err := uuid.Parse(s.GUID); err != nil {
		return errors.New("guid must be a valid UUID")
	}

	if s.LogicID == "" {
		return errors.New("logic_id is required")
	}
	if len(s.LogicID) > 128 {
		return errors.New("logic_id exceeds maximum length")
	}

	if s.Locked && s.LockInfo == nil {
		return errors.New("locked state must include lock_info")
	}
	if !s.Locked && s.LockInfo != nil {
		return errors.New("unlocked state cannot include lock_info")
	}

	return nil
}

// SizeExceedsThreshold reports whether the state content triggers a warning.
func (s *State) SizeExceedsThreshold() bool {
	return len(s.StateContent) > StateSizeWarningThreshold
}

package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const StateSizeWarningThreshold = 10 * 1024 * 1024 // 10MB

// State represents the persisted Terraform state
type State struct {
	bun.BaseModel `bun:"table:states,alias:s"`

	GUID         string    `bun:"guid,pk,type:uuid"`
	LogicID      string    `bun:"logic_id,notnull,unique"`
	StateContent []byte    `bun:"state_content,type:bytea"`
	Locked       bool      `bun:"locked,notnull,default:false"`
	LockInfo     *LockInfo `bun:"lock_info,type:jsonb"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

// LockInfo captures Terraform lock metadata stored with the state.
type LockInfo struct {
	ID        string    `json:"ID"`
	Operation string    `json:"Operation"`
	Info      string    `json:"Info"`
	Who       string    `json:"Who"`
	Version   string    `json:"Version"`
	Created   time.Time `json:"Created"`
	Path      string    `json:"Path"`
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

package state

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// BackendConfig represents the Terraform HTTP backend endpoints returned to clients.
type BackendConfig struct {
	Address       string
	LockAddress   string
	UnlockAddress string
}

// StateSummary captures state metadata for list operations.
type StateSummary struct {
	GUID      string
	LogicID   string
	Locked    bool
	SizeBytes int64
	CreatedAt time.Time
	UpdatedAt time.Time
	LockInfo  *models.LockInfo
}

// Service orchestrates state persistence and validation for RPC handlers.
type Service struct {
	repo      repository.StateRepository
	serverURL string
}

// NewService constructs a new Service instance.
func NewService(repo repository.StateRepository, serverURL string) *Service {
	return &Service{repo: repo, serverURL: serverURL}
}

// CreateState validates inputs, persists the state, and returns summary + backend config.
func (s *Service) CreateState(ctx context.Context, guid, logicID string) (*StateSummary, *BackendConfig, error) {
	if _, err := uuid.Parse(guid); err != nil {
		return nil, nil, fmt.Errorf("invalid GUID format: %w", err)
	}

	if err := validateLogicID(logicID); err != nil {
		return nil, nil, err
	}

	record := &models.State{
		GUID:    guid,
		LogicID: logicID,
	}

	if err := s.repo.Create(ctx, record); err != nil {
		return nil, nil, fmt.Errorf("create state: %w", err)
	}

	summary := toSummary(record)
	config := s.backendConfig(guid)
	return &summary, config, nil
}

// ListStates returns summaries for all states ordered newest first.
func (s *Service) ListStates(ctx context.Context) ([]StateSummary, error) {
	records, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list states: %w", err)
	}

	summaries := make([]StateSummary, 0, len(records))
	for _, rec := range records {
		recordCopy := rec
		summaries = append(summaries, toSummary(&recordCopy))
	}

	return summaries, nil
}

// GetStateConfig resolves backend configuration for a state by logic ID.
func (s *Service) GetStateConfig(ctx context.Context, logicID string) (string, *BackendConfig, error) {
	if err := validateLogicID(logicID); err != nil {
		return "", nil, err
	}

	record, err := s.repo.GetByLogicID(ctx, logicID)
	if err != nil {
		return "", nil, fmt.Errorf("get state: %w", err)
	}

	config := s.backendConfig(record.GUID)
	return record.GUID, config, nil
}

// GetStateByGUID retrieves a state record by GUID for direct access
func (s *Service) GetStateByGUID(ctx context.Context, guid string) (*models.State, error) {
	record, err := s.repo.GetByGUID(ctx, guid)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	return record, nil
}

// GetStateLock returns the current lock metadata for a state.
func (s *Service) GetStateLock(ctx context.Context, guid string) (*models.LockInfo, error) {
	record, err := s.repo.GetByGUID(ctx, guid)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}

	return record.LockInfo, nil
}

// UnlockState releases the lock using the provided lock ID verification.
func (s *Service) UnlockState(ctx context.Context, guid, lockID string) error {
	if lockID == "" {
		return fmt.Errorf("lock_id is required")
	}

	if err := s.repo.Unlock(ctx, guid, lockID); err != nil {
		return fmt.Errorf("unlock state: %w", err)
	}

	return nil
}

// UpdateStateContent replaces the stored Terraform state payload.
// If lockID is provided and matches the current lock, the update is allowed even when locked.
func (s *Service) UpdateStateContent(ctx context.Context, guid string, content []byte, lockID string) (*StateSummary, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("state content must not be empty")
	}

	record, err := s.repo.GetByGUID(ctx, guid)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}

	// If state is locked, verify the caller holds the lock
	if record.Locked {
		if lockID == "" || record.LockInfo == nil || lockID != record.LockInfo.ID {
			return nil, fmt.Errorf("state is locked, cannot update")
		}
		// Lock ID matches, allow the update
	}

	record.StateContent = content
	if err := s.repo.Update(ctx, record); err != nil {
		return nil, fmt.Errorf("update state content: %w", err)
	}

	summary := toSummary(record)
	return &summary, nil
}

// LockState acquires a lock for the given state.
func (s *Service) LockState(ctx context.Context, guid string, lockInfo *models.LockInfo) error {
	if lockInfo == nil {
		return fmt.Errorf("lock info is required")
	}
	if lockInfo.Created.IsZero() {
		lockInfo.Created = time.Now()
	}

	if err := s.repo.Lock(ctx, guid, lockInfo); err != nil {
		return fmt.Errorf("lock state: %w", err)
	}

	return nil
}

func (s *Service) backendConfig(guid string) *BackendConfig {
	return &BackendConfig{
		Address:       fmt.Sprintf("%s/tfstate/%s", s.serverURL, guid),
		LockAddress:   fmt.Sprintf("%s/tfstate/%s/lock", s.serverURL, guid),
		UnlockAddress: fmt.Sprintf("%s/tfstate/%s/unlock", s.serverURL, guid),
	}
}

func toSummary(record *models.State) StateSummary {
	var size int64
	if record.StateContent != nil {
		size = int64(len(record.StateContent))
	}

	return StateSummary{
		GUID:      record.GUID,
		LogicID:   record.LogicID,
		Locked:    record.Locked,
		LockInfo:  record.LockInfo,
		SizeBytes: size,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func validateLogicID(logicID string) error {
	if logicID == "" {
		return fmt.Errorf("logic_id is required")
	}
	if len(logicID) > 128 {
		return fmt.Errorf("logic_id exceeds maximum length of 128 characters")
	}
	return nil
}

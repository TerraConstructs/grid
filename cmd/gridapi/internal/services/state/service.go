package state

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/tfstate"
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
	Labels    models.LabelMap
}

// StateInfo provides comprehensive state information including dependencies, dependents, and outputs.
type StateInfo struct {
	GUID          string
	LogicID       string
	BackendConfig *BackendConfig
	Dependencies  []models.Edge
	Dependents    []models.Edge
	Outputs       []repository.OutputKey
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SizeBytes     int64
	Labels        models.LabelMap
}

// Service orchestrates state persistence and validation for RPC handlers.
type Service struct {
	repo       repository.StateRepository
	outputRepo repository.StateOutputRepository
	edgeRepo   repository.EdgeRepository
	policyRepo repository.LabelPolicyRepository
	serverURL  string
}

// NewService constructs a new Service instance.
func NewService(repo repository.StateRepository, serverURL string) *Service {
	return &Service{repo: repo, serverURL: serverURL}
}

// WithOutputRepository adds the output repository to the service (optional dependency).
func (s *Service) WithOutputRepository(outputRepo repository.StateOutputRepository) *Service {
	s.outputRepo = outputRepo
	return s
}

// WithEdgeRepository adds the edge repository to the service (optional dependency).
func (s *Service) WithEdgeRepository(edgeRepo repository.EdgeRepository) *Service {
	s.edgeRepo = edgeRepo
	return s
}

// WithPolicyRepository adds the policy repository to the service (optional dependency).
func (s *Service) WithPolicyRepository(policyRepo repository.LabelPolicyRepository) *Service {
	s.policyRepo = policyRepo
	return s
}

// CreateState validates inputs, persists the state, and returns summary + backend config.
// T033: Updated to accept and validate labels via LabelValidator.
func (s *Service) CreateState(ctx context.Context, guid, logicID string, labels models.LabelMap) (*StateSummary, *BackendConfig, error) {
	if _, err := uuid.Parse(guid); err != nil {
		return nil, nil, fmt.Errorf("invalid GUID format: %w", err)
	}

	if err := validateLogicID(logicID); err != nil {
		return nil, nil, err
	}

	// Validate labels against policy (if labels provided)
	if len(labels) > 0 {
		validator := NewLabelValidator(nil)
		if s.policyRepo != nil {
			if policy, err := s.policyRepo.GetPolicy(ctx); err == nil {
				validator = NewLabelValidator(&policy.PolicyJSON)
			}
		}
		if err := validator.Validate(labels); err != nil {
			return nil, nil, fmt.Errorf("label validation failed: %w", err)
		}
	}

	record := &models.State{
		GUID:    guid,
		LogicID: logicID,
		Labels:  labels,
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

// ListStatesWithFilter returns states matching bexpr filter with pagination.
func (s *Service) ListStatesWithFilter(ctx context.Context, filter string, pageSize int, offset int) ([]StateSummary, error) {
	states, err := s.repo.ListWithFilter(ctx, filter, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list states with filter: %w", err)
	}

	summaries := make([]StateSummary, 0, len(states))
	for _, state := range states {
		stateCopy := state
		summaries = append(summaries, toSummary(&stateCopy))
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

// GetEdgeByID retrieves an edge by ID for authorization checks
func (s *Service) GetEdgeByID(ctx context.Context, edgeID int64) (*models.Edge, error) {
	edge, err := s.edgeRepo.GetByID(ctx, edgeID)
	if err != nil {
		return nil, fmt.Errorf("get edge: %w", err)
	}
	return edge, nil
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

// StateUpdateResult contains the result of a state content update
type StateUpdateResult struct {
	Summary      *StateSummary
	OutputValues map[string]interface{} // Parsed output values for edge job
}

// UpdateStateContent replaces the stored Terraform state payload.
// If lockID is provided and matches the current lock, the update is allowed even when locked.
// Updates the output cache atomically in the same transaction (FR-027 compliance).
// Returns parsed output values to avoid double-parsing for edge updates.
func (s *Service) UpdateStateContent(ctx context.Context, guid string, content []byte, lockID string) (*StateUpdateResult, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("state content must not be empty")
	}

	// Parse state once to get serial, keys, and values
	parsed, err := tfstate.ParseState(content)
	if err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}

	// Use atomic update method to ensure state and outputs are consistent (FR-027)
	// Both operations happen in ONE transaction via repository.UpdateContentAndUpsertOutputs
	err = s.repo.UpdateContentAndUpsertOutputs(ctx, guid, content, lockID, parsed.Serial, parsed.Keys)
	if err != nil {
		return nil, fmt.Errorf("update state content: %w", err)
	}

	// Fetch updated state for summary
	record, err := s.repo.GetByGUID(ctx, guid)
	if err != nil {
		return nil, fmt.Errorf("get updated state: %w", err)
	}

	summary := toSummary(record)
	return &StateUpdateResult{
		Summary:      &summary,
		OutputValues: parsed.Values,
	}, nil
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
	size := record.SizeBytes
	if size == 0 && record.StateContent != nil {
		size = int64(len(record.StateContent))
	}

	var labels models.LabelMap
	if record.Labels != nil {
		labels = make(models.LabelMap, len(record.Labels))
		for key, value := range record.Labels {
			labels[key] = value
		}
	}

	return StateSummary{
		GUID:      record.GUID,
		LogicID:   record.LogicID,
		Locked:    record.Locked,
		LockInfo:  record.LockInfo,
		SizeBytes: size,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		Labels:    labels,
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

// GetOutputKeys retrieves output keys with sensitive flags from cached state outputs.
// Falls back to parsing state JSON if cache is unavailable or empty.
func (s *Service) GetOutputKeys(ctx context.Context, guid string) ([]repository.OutputKey, error) {
	// Try cache first if output repository is available
	if s.outputRepo != nil {
		cached, err := s.outputRepo.GetOutputsByState(ctx, guid)
		if err == nil && len(cached) > 0 {
			return cached, nil
		}
		// Continue to fallback if cache is empty or error occurred
	}

	// Fallback: fetch state and parse outputs from state JSON
	record, err := s.repo.GetByGUID(ctx, guid)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}

	// Parse output keys from state content (if available)
	if len(record.StateContent) == 0 {
		return []repository.OutputKey{}, nil
	}

	// Parse outputs from Terraform state JSON
	tfOutputs, err := tfstate.ParseOutputKeys(record.StateContent)
	if err != nil {
		return nil, fmt.Errorf("parse output keys: %w", err)
	}

	// tfstate.OutputKey is already repository.OutputKey and can return directly
	return tfOutputs, nil
}

// UpdateLabels modifies labels on a state with atomic updates and policy validation.
// T032: Implements add/remove operations with validation and updated_at bump.
// Accepts adds (key-value pairs to add/update) and removals (keys to remove).
func (s *Service) UpdateLabels(ctx context.Context, guid string, adds models.LabelMap, removals []string) error {
	// Fetch current state
	state, err := s.repo.GetByGUID(ctx, guid)
	if err != nil {
		return fmt.Errorf("get state: %w", err)
	}

	// Initialize labels if nil
	if state.Labels == nil {
		state.Labels = make(models.LabelMap)
	}

	// Apply adds (merge into existing labels)
	for k, v := range adds {
		state.Labels[k] = v
	}

	// Apply removals
	for _, k := range removals {
		delete(state.Labels, k)
	}

	// Validate updated labels against policy (fallback to basic validation if policy unavailable)
	validator := NewLabelValidator(nil)
	if s.policyRepo != nil {
		if policy, err := s.policyRepo.GetPolicy(ctx); err == nil {
			validator = NewLabelValidator(&policy.PolicyJSON)
		}
	}
	if err := validator.Validate(state.Labels); err != nil {
		return fmt.Errorf("label validation failed: %w", err)
	}

	// Update state (repository will bump updated_at)
	if err := s.repo.Update(ctx, state); err != nil {
		return fmt.Errorf("update state: %w", err)
	}

	return nil
}

// GetStateInfo retrieves comprehensive state information including dependencies, dependents, and outputs.
// This consolidates multiple data sources into a single response using eager loading to avoid N+1 queries.
func (s *Service) GetStateInfo(ctx context.Context, logicID, guid string) (*StateInfo, error) {
	// First resolve GUID if only logic_id provided
	resolvedGUID := guid
	if logicID != "" && guid == "" {
		state, err := s.repo.GetByLogicID(ctx, logicID)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}
		resolvedGUID = state.GUID
	} else if guid == "" {
		return nil, fmt.Errorf("state reference required (logic_id or guid)")
	}

	// Fetch state with all related data in one query using eager loading
	state, err := s.repo.GetByGUIDWithRelations(ctx, resolvedGUID, "Outputs", "IncomingEdges", "OutgoingEdges")
	if err != nil {
		return nil, fmt.Errorf("get state with relations: %w", err)
	}

	// Build state info
	info := &StateInfo{
		GUID:          state.GUID,
		LogicID:       state.LogicID,
		BackendConfig: s.backendConfig(state.GUID),
		CreatedAt:     state.CreatedAt,
		UpdatedAt:     state.UpdatedAt,
		SizeBytes:     state.SizeBytes,
		Labels:        state.Labels,
	}

	// Convert eagerly loaded outputs to OutputKey slice
	if state.Outputs != nil {
		outputs := make([]repository.OutputKey, len(state.Outputs))
		for i, out := range state.Outputs {
			outputs[i] = repository.OutputKey{
				Key:       out.OutputKey,
				Sensitive: out.Sensitive,
			}
		}
		info.Outputs = outputs
	}

	// Convert eagerly loaded edges to models.Edge slices
	if state.IncomingEdges != nil {
		dependencies := make([]models.Edge, len(state.IncomingEdges))
		for i, edge := range state.IncomingEdges {
			dependencies[i] = *edge
		}
		info.Dependencies = dependencies
	}

	if state.OutgoingEdges != nil {
		dependents := make([]models.Edge, len(state.OutgoingEdges))
		for i, edge := range state.OutgoingEdges {
			dependents[i] = *edge
		}
		info.Dependents = dependents
	}

	return info, nil
}

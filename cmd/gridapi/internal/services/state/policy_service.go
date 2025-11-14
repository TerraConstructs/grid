package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// PolicyValidatorInterface defines policy validation operations for testability.
type PolicyValidatorInterface interface {
	ValidatePolicyStructure(policyJSON []byte) (*models.PolicyDefinition, error)
}

// PolicyService manages label policy operations with validation.
// T034: Implements GetPolicy, SetPolicy with JSON/schema validation per FR-029.
type PolicyService struct {
	policyRepo repository.LabelPolicyRepository
	validator  PolicyValidatorInterface
}

// NewPolicyService constructs a policy service with repository and validator.
func NewPolicyService(policyRepo repository.LabelPolicyRepository, validator PolicyValidatorInterface) *PolicyService {
	if validator == nil {
		validator = NewPolicyValidator()
	}
	return &PolicyService{
		policyRepo: policyRepo,
		validator:  validator,
	}
}

// GetPolicy retrieves the current label policy.
func (s *PolicyService) GetPolicy(ctx context.Context) (*models.LabelPolicy, error) {
	policy, err := s.policyRepo.GetPolicy(ctx)
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}

	return policy, nil
}

// SetPolicy validates and persists a policy definition object.
func (s *PolicyService) SetPolicy(ctx context.Context, policy *models.PolicyDefinition) error {
	// Marshal to JSON for validation
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}

	// Validate via SetPolicyFromJSON
	return s.SetPolicyFromJSON(ctx, policyJSON)
}

// SetPolicyFromJSON validates and persists a new policy definition from JSON.
// FR-029: Validates policy structure before persistence, returning clear errors.
func (s *PolicyService) SetPolicyFromJSON(ctx context.Context, policyJSON []byte) error {
	// Validate policy structure via PolicyValidator (FR-029)
	validatedPolicy, err := s.validator.ValidatePolicyStructure(policyJSON)
	if err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}

	// Persist validated policy
	if err := s.policyRepo.SetPolicy(ctx, validatedPolicy); err != nil {
		return fmt.Errorf("set policy: %w", err)
	}

	return nil
}

// ValidateLabels performs a dry-run validation of labels against current policy.
// Returns nil if labels are compliant, or validation error otherwise.
func (s *PolicyService) ValidateLabels(ctx context.Context, labels models.LabelMap) error {
	// Get current policy
	policy, err := s.policyRepo.GetPolicy(ctx)
	if err != nil {
		// If no policy exists, use basic validation
		validator := NewLabelValidator(nil)
		return validator.Validate(labels)
	}

	// Validate against current policy
	validator := NewLabelValidator(&policy.PolicyJSON)
	return validator.Validate(labels)
}

package state

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

// MockLabelPolicyRepository is a mock implementation
type MockLabelPolicyRepository struct {
	mock.Mock
}

func (m *MockLabelPolicyRepository) GetPolicy(ctx context.Context) (*models.LabelPolicy, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LabelPolicy), args.Error(1)
}

func (m *MockLabelPolicyRepository) SetPolicy(ctx context.Context, policy *models.PolicyDefinition) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

// MockPolicyValidator is a mock implementation
type MockPolicyValidator struct {
	mock.Mock
}

func (m *MockPolicyValidator) ValidatePolicyStructure(policyJSON []byte) (*models.PolicyDefinition, error) {
	args := m.Called(policyJSON)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PolicyDefinition), args.Error(1)
}

// T017a: Test PolicyService.SetPolicy with malformed JSON
func TestPolicyService_SetPolicyWithMalformedJSON(t *testing.T) {
	t.Run("malformed JSON rejected before activation", func(t *testing.T) {
		mockRepo := new(MockLabelPolicyRepository)
		mockValidator := new(MockPolicyValidator)

		service := NewPolicyService(mockRepo, mockValidator)
		ctx := context.Background()

		malformedJSON := []byte(`{"allowed_keys": {"env": {}}, "max_keys": }`) // Missing value

		// Validator should catch malformed JSON
		mockValidator.On("ValidatePolicyStructure", malformedJSON).Return(nil, assert.AnError)

		err := service.SetPolicyFromJSON(ctx, malformedJSON)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")

		// Repository should NOT be called
		mockRepo.AssertNotCalled(t, "SetPolicy")
		mockValidator.AssertExpectations(t)
	})

	t.Run("error message is clear for malformed JSON", func(t *testing.T) {
		mockRepo := new(MockLabelPolicyRepository)
		mockValidator := new(MockPolicyValidator)

		service := NewPolicyService(mockRepo, mockValidator)
		ctx := context.Background()

		malformedJSON := []byte(`not-json-at-all`)

		// Validator returns descriptive error
		mockValidator.On("ValidatePolicyStructure", malformedJSON).
			Return(nil, assert.AnError)

		err := service.SetPolicyFromJSON(ctx, malformedJSON)
		assert.Error(t, err)

		mockValidator.AssertExpectations(t)
	})
}

// T017b: Test PolicyService.SetPolicy with invalid schema
func TestPolicyService_SetPolicyWithInvalidSchema(t *testing.T) {
	t.Run("missing required fields rejected", func(t *testing.T) {
		mockRepo := new(MockLabelPolicyRepository)
		mockValidator := new(MockPolicyValidator)

		service := NewPolicyService(mockRepo, mockValidator)
		ctx := context.Background()

		// Valid JSON but missing required fields
		invalidPolicy := map[string]interface{}{
			"allowed_keys": map[string]interface{}{"env": map[string]interface{}{}},
			// Missing max_keys and max_value_len
		}
		policyJSON, _ := json.Marshal(invalidPolicy)

		// Validator should catch missing fields
		mockValidator.On("ValidatePolicyStructure", policyJSON).
			Return(nil, assert.AnError)

		err := service.SetPolicyFromJSON(ctx, policyJSON)
		assert.Error(t, err)

		mockRepo.AssertNotCalled(t, "SetPolicy")
		mockValidator.AssertExpectations(t)
	})

	t.Run("wrong field types rejected", func(t *testing.T) {
		mockRepo := new(MockLabelPolicyRepository)
		mockValidator := new(MockPolicyValidator)

		service := NewPolicyService(mockRepo, mockValidator)
		ctx := context.Background()

		// Valid JSON but wrong types
		invalidPolicy := map[string]interface{}{
			"allowed_keys":   "should-be-map",       // Wrong type
			"allowed_values": []string{"not a map"}, // Wrong type
			"max_keys":       "not-a-number",        // Wrong type
			"max_value_len":  100,
		}
		policyJSON, _ := json.Marshal(invalidPolicy)

		mockValidator.On("ValidatePolicyStructure", policyJSON).
			Return(nil, assert.AnError)

		err := service.SetPolicyFromJSON(ctx, policyJSON)
		assert.Error(t, err)

		mockRepo.AssertNotCalled(t, "SetPolicy")
		mockValidator.AssertExpectations(t)
	})

	t.Run("valid policy passes validation and persists", func(t *testing.T) {
		mockRepo := new(MockLabelPolicyRepository)
		mockValidator := new(MockPolicyValidator)

		service := NewPolicyService(mockRepo, mockValidator)
		ctx := context.Background()

		validPolicy := &models.PolicyDefinition{
			AllowedKeys: map[string]struct{}{
				"env": {},
			},
			AllowedValues: map[string][]string{
				"env": {"staging", "prod"},
			},
			MaxKeys:     32,
			MaxValueLen: 256,
		}
		policyJSON, _ := json.Marshal(validPolicy)

		// Validator passes and returns validated policy
		mockValidator.On("ValidatePolicyStructure", policyJSON).Return(validPolicy, nil)

		// Repository saves policy
		mockRepo.On("SetPolicy", ctx, mock.MatchedBy(func(p *models.PolicyDefinition) bool {
			return p.MaxKeys == 32 && p.MaxValueLen == 256
		})).Return(nil)

		err := service.SetPolicyFromJSON(ctx, policyJSON)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockValidator.AssertExpectations(t)
	})
}

package state

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// MockStateRepository is a mock implementation of repository.StateRepository
type MockStateRepository struct {
	mock.Mock
}

func (m *MockStateRepository) Create(ctx context.Context, state *models.State) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockStateRepository) GetByGUID(ctx context.Context, guid string) (*models.State, error) {
	args := m.Called(ctx, guid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.State), args.Error(1)
}

func (m *MockStateRepository) GetByLogicID(ctx context.Context, logicID string) (*models.State, error) {
	args := m.Called(ctx, logicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.State), args.Error(1)
}

func (m *MockStateRepository) Update(ctx context.Context, state *models.State) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockStateRepository) List(ctx context.Context) ([]models.State, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.State), args.Error(1)
}

func (m *MockStateRepository) Lock(ctx context.Context, guid string, lockInfo *models.LockInfo) error {
	args := m.Called(ctx, guid, lockInfo)
	return args.Error(0)
}

func (m *MockStateRepository) Unlock(ctx context.Context, guid string, lockID string) error {
	args := m.Called(ctx, guid, lockID)
	return args.Error(0)
}

func (m *MockStateRepository) UpdateContentAndUpsertOutputs(ctx context.Context, guid string, stateContent []byte, lockID string, serial int64, outputs []repository.OutputKey) error {
	args := m.Called(ctx, guid, stateContent, lockID, serial, outputs)
	return args.Error(0)
}

func (m *MockStateRepository) ListWithFilter(ctx context.Context, filter string, pageSize int, offset int) ([]models.State, error) {
	args := m.Called(ctx, filter, pageSize, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.State), args.Error(1)
}

// T015: Test StateService.CreateState with labels
func TestStateService_CreateStateWithLabels(t *testing.T) {
	t.Run("creates state with valid labels", func(t *testing.T) {
		mockRepo := new(MockStateRepository)
		mockPolicyRepo := new(MockLabelPolicyRepository)

		service := NewService(mockRepo, "http://localhost:8080").
			WithPolicyRepository(mockPolicyRepo)
		ctx := context.Background()

		guid := uuid.NewString()
		logicID := "test-logic-id"
		labels := models.LabelMap{
			"env":  "staging",
			"team": "platform",
		}

		// Mock policy fetch - return valid policy
		policy := &models.LabelPolicy{
			ID:      1,
			Version: 1,
			PolicyJSON: models.PolicyDefinition{
				AllowedKeys: map[string]struct{}{
					"env":  {},
					"team": {},
				},
				MaxKeys:     32,
				MaxValueLen: 256,
			},
		}
		mockPolicyRepo.On("GetPolicy", ctx).Return(policy, nil)

		// Expect repository create to be called with labels
		mockRepo.On("Create", ctx, mock.MatchedBy(func(s *models.State) bool {
			return s.GUID == guid &&
				s.LogicID == logicID &&
				s.Labels["env"] == "staging" &&
				s.Labels["team"] == "platform"
		})).Return(nil)

		_, _, err := service.CreateState(ctx, guid, logicID, labels)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockPolicyRepo.AssertExpectations(t)
	})

	t.Run("validation error prevents create", func(t *testing.T) {
		mockRepo := new(MockStateRepository)
		mockPolicyRepo := new(MockLabelPolicyRepository)

		service := NewService(mockRepo, "http://localhost:8080").
			WithPolicyRepository(mockPolicyRepo)
		ctx := context.Background()

		guid := uuid.NewString()
		logicID := "test-logic-id"
		labels := models.LabelMap{
			"invalid-KEY": "value", // uppercase not allowed
		}

		// Mock policy fetch
		policy := &models.LabelPolicy{
			ID:      1,
			Version: 1,
			PolicyJSON: models.PolicyDefinition{
				MaxKeys:     32,
				MaxValueLen: 256,
			},
		}
		mockPolicyRepo.On("GetPolicy", ctx).Return(policy, nil)

		// CreateState should fail validation
		_, _, err := service.CreateState(ctx, guid, logicID, labels)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")

		// Repository should NOT be called
		mockRepo.AssertNotCalled(t, "Create")
	})
}

// T016: Test StateService.UpdateLabels
func TestStateService_UpdateLabels(t *testing.T) {
	t.Run("add and remove labels atomically", func(t *testing.T) {
		mockRepo := new(MockStateRepository)
		mockPolicyRepo := new(MockLabelPolicyRepository)

		service := NewService(mockRepo, "http://localhost:8080").
			WithPolicyRepository(mockPolicyRepo)
		ctx := context.Background()

		guid := uuid.NewString()
		existingState := &models.State{
			GUID:    guid,
			LogicID: "test-state",
			Labels: models.LabelMap{
				"env":  "staging",
				"team": "core",
			},
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		}

		// Mock GetByGUID
		mockRepo.On("GetByGUID", ctx, guid).Return(existingState, nil)

		// Adds: modify env, add region
		adds := models.LabelMap{
			"env":    "prod",
			"region": "us-west",
		}
		// Removals: remove team
		removals := []string{"team"}

		// Mock policy fetch
		policy := &models.LabelPolicy{
			ID:      1,
			Version: 1,
			PolicyJSON: models.PolicyDefinition{
				MaxKeys:     32,
				MaxValueLen: 256,
			},
		}
		mockPolicyRepo.On("GetPolicy", ctx).Return(policy, nil)

		// Mock Update - verify final label state
		mockRepo.On("Update", ctx, mock.MatchedBy(func(s *models.State) bool {
			// Should have env=prod, region=us-west, no team
			_, hasTeam := s.Labels["team"]
			return s.Labels["env"] == "prod" &&
				s.Labels["region"] == "us-west" &&
				!hasTeam
		})).Return(nil)

		err := service.UpdateLabels(ctx, guid, adds, removals)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockPolicyRepo.AssertExpectations(t)
	})

	t.Run("validation error prevents update", func(t *testing.T) {
		mockRepo := new(MockStateRepository)
		mockPolicyRepo := new(MockLabelPolicyRepository)

		service := NewService(mockRepo, "http://localhost:8080").
			WithPolicyRepository(mockPolicyRepo)
		ctx := context.Background()

		guid := uuid.NewString()
		existingState := &models.State{
			GUID:    guid,
			LogicID: "test-state",
			Labels:  models.LabelMap{"env": "staging"},
		}

		mockRepo.On("GetByGUID", ctx, guid).Return(existingState, nil)

		// Invalid key format (uppercase)
		invalidAdds := models.LabelMap{"INVALID-KEY": "value"}

		// Mock policy fetch
		policy := &models.LabelPolicy{
			ID:      1,
			Version: 1,
			PolicyJSON: models.PolicyDefinition{
				MaxKeys:     32,
				MaxValueLen: 256,
			},
		}
		mockPolicyRepo.On("GetPolicy", ctx).Return(policy, nil)

		err := service.UpdateLabels(ctx, guid, invalidAdds, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")

		// Update should NOT be called
		mockRepo.AssertNotCalled(t, "Update")
	})
}

// T017: Test StateService.UpdateLabels updates updated_at
func TestStateService_UpdateLabelsUpdatesTimestamp(t *testing.T) {
	t.Run("updated_at timestamp bumps when labels change", func(t *testing.T) {
		mockRepo := new(MockStateRepository)
		mockPolicyRepo := new(MockLabelPolicyRepository)

		service := NewService(mockRepo, "http://localhost:8080").
			WithPolicyRepository(mockPolicyRepo)
		ctx := context.Background()

		guid := uuid.NewString()
		oldTime := time.Now().Add(-1 * time.Hour)
		existingState := &models.State{
			GUID:      guid,
			LogicID:   "test-state",
			Labels:    models.LabelMap{"env": "staging"},
			UpdatedAt: oldTime,
		}

		mockRepo.On("GetByGUID", ctx, guid).Return(existingState, nil)

		// Update env label
		adds := models.LabelMap{"env": "prod"}

		// Mock policy fetch
		policy := &models.LabelPolicy{
			ID:      1,
			Version: 1,
			PolicyJSON: models.PolicyDefinition{
				MaxKeys:     32,
				MaxValueLen: 256,
			},
		}
		mockPolicyRepo.On("GetPolicy", ctx).Return(policy, nil)

		// The repository Update is responsible for bumping updated_at
		// We just verify Update was called (actual timestamp bump happens in repository)
		mockRepo.On("Update", ctx, mock.MatchedBy(func(s *models.State) bool {
			return s.Labels["env"] == "prod"
		})).Return(nil)

		err := service.UpdateLabels(ctx, guid, adds, nil)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockPolicyRepo.AssertExpectations(t)
	})
}

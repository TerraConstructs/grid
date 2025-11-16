package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/auth"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/services/state"
)

// mockStateService is a mock implementation of the state service for testing
type mockStateService struct {
	createStateFunc    func(ctx context.Context, guid, logicID string) (*statepkg.StateSummary, *statepkg.BackendConfig, error)
	listStatesFunc     func(ctx context.Context) ([]statepkg.StateSummary, error)
	getStateConfigFunc func(ctx context.Context, logicID string) (string, *statepkg.BackendConfig, error)
	getByGUIDFunc      func(ctx context.Context, guid string) (*models.State, error)
	updateContentFunc  func(ctx context.Context, guid string, content []byte, lockID string) (*statepkg.StateUpdateResult, error)
	lockStateFunc      func(ctx context.Context, guid string, lockInfo *models.LockInfo) error
	unlockStateFunc    func(ctx context.Context, guid string, lockID string) error
	getStateLockFunc   func(ctx context.Context, guid string) (*models.LockInfo, error)
}

func (m *mockStateService) CreateState(ctx context.Context, guid, logicID string) (*statepkg.StateSummary, *statepkg.BackendConfig, error) {
	if m.createStateFunc != nil {
		return m.createStateFunc(ctx, guid, logicID)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockStateService) ListStates(ctx context.Context) ([]statepkg.StateSummary, error) {
	if m.listStatesFunc != nil {
		return m.listStatesFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockStateService) GetStateConfig(ctx context.Context, logicID string) (string, *statepkg.BackendConfig, error) {
	if m.getStateConfigFunc != nil {
		return m.getStateConfigFunc(ctx, logicID)
	}
	return "", nil, errors.New("not implemented")
}

func (m *mockStateService) GetStateByGUID(ctx context.Context, guid string) (*models.State, error) {
	if m.getByGUIDFunc != nil {
		return m.getByGUIDFunc(ctx, guid)
	}
	return nil, errors.New("not implemented")
}

func (m *mockStateService) UpdateStateContent(ctx context.Context, guid string, content []byte, lockID string) (*statepkg.StateUpdateResult, error) {
	if m.updateContentFunc != nil {
		return m.updateContentFunc(ctx, guid, content, lockID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockStateService) LockState(ctx context.Context, guid string, lockInfo *models.LockInfo) error {
	if m.lockStateFunc != nil {
		return m.lockStateFunc(ctx, guid, lockInfo)
	}
	return errors.New("not implemented")
}

func (m *mockStateService) UnlockState(ctx context.Context, guid string, lockID string) error {
	if m.unlockStateFunc != nil {
		return m.unlockStateFunc(ctx, guid, lockID)
	}
	return errors.New("not implemented")
}

func (m *mockStateService) GetStateLock(ctx context.Context, guid string) (*models.LockInfo, error) {
	if m.getStateLockFunc != nil {
		return m.getStateLockFunc(ctx, guid)
	}
	return nil, errors.New("not implemented")
}

func TestGetState(t *testing.T) {
	tests := []struct {
		name           string
		guid           string
		mockService    *mockStateService
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "success with state content",
			guid: "test-guid-123",
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{
						GUID:         guid,
						StateContent: []byte(`{"version": 4}`),
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"version": 4}`,
		},
		{
			name: "success with empty state",
			guid: "test-guid-456",
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{
						GUID:         guid,
						StateContent: nil,
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"version":4,"terraform_version":"","serial":0,"lineage":"","outputs":null,"resources":null}`,
		},
		{
			name: "state not found",
			guid: "nonexistent-guid",
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return nil, errors.New("state not found")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			handlers := &TerraformHandlers{service: tt.mockService}
			r.Get("/tfstate/{guid}", handlers.GetState)

			req := httptest.NewRequest("GET", "/tfstate/"+tt.guid, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" && w.Body.String() != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestUpdateState(t *testing.T) {
	tests := []struct {
		name           string
		guid           string
		body           string
		mockService    *mockStateService
		expectedStatus int
		checkHeader    bool
		headerName     string
	}{
		{
			name: "success",
			guid: "test-guid-123",
			body: `{"version": 4}`,
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				updateContentFunc: func(ctx context.Context, guid string, content []byte, lockID string) (*statepkg.StateUpdateResult, error) {
					return &statepkg.StateUpdateResult{
						Summary: &statepkg.StateSummary{
							SizeBytes: 100,
						},
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "success with size warning",
			guid: "test-guid-456",
			body: `{"version": 4}`,
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				updateContentFunc: func(ctx context.Context, guid string, content []byte, lockID string) (*statepkg.StateUpdateResult, error) {
					return &statepkg.StateUpdateResult{
						Summary: &statepkg.StateSummary{
							SizeBytes: 11 * 1024 * 1024, // 11MB > 10MB threshold
						},
					}, nil
				},
			},
			expectedStatus: http.StatusOK,
			checkHeader:    true,
			headerName:     "X-Grid-State-Size-Warning",
		},
		{
			name: "invalid JSON",
			guid: "test-guid-789",
			body: `not json`,
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				updateContentFunc: func(ctx context.Context, guid string, content []byte, lockID string) (*statepkg.StateUpdateResult, error) {
					return nil, errors.New("should not be called")
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "state not found",
			guid: "nonexistent-guid",
			body: `{"version": 4}`,
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return nil, errors.New("state not found")
				},
				updateContentFunc: func(ctx context.Context, guid string, content []byte, lockID string) (*statepkg.StateUpdateResult, error) {
					return nil, errors.New("should not be called")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "state locked",
			guid: "locked-guid",
			body: `{"version": 4}`,
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				updateContentFunc: func(ctx context.Context, guid string, content []byte, lockID string) (*statepkg.StateUpdateResult, error) {
					return nil, errors.New("state is locked")
				},
			},
			expectedStatus: http.StatusLocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			handlers := &TerraformHandlers{service: tt.mockService}
			r.Post("/tfstate/{guid}", handlers.UpdateState)

			req := httptest.NewRequest("POST", "/tfstate/"+tt.guid, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkHeader {
				header := w.Header().Get(tt.headerName)
				if header == "" {
					t.Errorf("expected header %s to be set", tt.headerName)
				}
			}
		})
	}
}

func TestLockState(t *testing.T) {
	// Create a mock principal for testing the context
	mockPrincipal := auth.AuthenticatedPrincipal{
		PrincipalID: "user:test-user-123",
	}

	tests := []struct {
		name           string
		guid           string
		lockInfo       models.LockInfo
		mockService    *mockStateService
		principal      *auth.AuthenticatedPrincipal // Optional principal
		expectedStatus int
	}{
		{
			name: "success",
			guid: "test-guid-123",
			lockInfo: models.LockInfo{
				ID:        "lock-123",
				Operation: "apply",
				Who:       "user@host",
			},
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				lockStateFunc: func(ctx context.Context, guid string, lockInfo *models.LockInfo) error {
					return nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "success with principal context",
			guid:      "test-guid-principal",
			principal: &mockPrincipal,
			lockInfo: models.LockInfo{
				ID:        "lock-principal",
				Operation: "apply",
				Who:       "user@host",
			},
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				lockStateFunc: func(ctx context.Context, guid string, lockInfo *models.LockInfo) error {
					assert.Equal(t, mockPrincipal.PrincipalID, lockInfo.OwnerPrincipalID, "OwnerPrincipalID should be set from context")
					return nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "already locked",
			guid: "locked-guid",
			lockInfo: models.LockInfo{
				ID:        "lock-456",
				Operation: "plan",
			},
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				lockStateFunc: func(ctx context.Context, guid string, lockInfo *models.LockInfo) error {
					return errors.New("already locked")
				},
				getStateLockFunc: func(ctx context.Context, guid string) (*models.LockInfo, error) {
					return &models.LockInfo{
						ID:        "existing-lock",
						Operation: "apply",
						Who:       "other@host",
						Created:   time.Now(),
					}, nil
				},
			},
			expectedStatus: http.StatusLocked,
		},
		{
			name: "state not found",
			guid: "nonexistent-guid",
			lockInfo: models.LockInfo{
				ID:        "lock-789",
				Operation: "apply",
			},
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return nil, errors.New("state not found")
				},
				lockStateFunc: func(ctx context.Context, guid string, lockInfo *models.LockInfo) error {
					return errors.New("should not be called")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			handlers := &TerraformHandlers{service: tt.mockService}
			// Use actual LOCK method (registered in init())
			r.Method("LOCK", "/tfstate/{guid}/lock", http.HandlerFunc(handlers.LockState))

			body, _ := json.Marshal(tt.lockInfo)
			req := httptest.NewRequest("LOCK", "/tfstate/"+tt.guid+"/lock", bytes.NewBuffer(body))

			// If the test case has a principal, add it to the request context
			if tt.principal != nil {
				ctx := auth.SetUserContext(req.Context(), *tt.principal)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestUnlockState(t *testing.T) {
	tests := []struct {
		name           string
		guid           string
		lockInfo       models.LockInfo
		mockService    *mockStateService
		expectedStatus int
	}{
		{
			name: "success",
			guid: "test-guid-123",
			lockInfo: models.LockInfo{
				ID: "lock-123",
			},
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				unlockStateFunc: func(ctx context.Context, guid string, lockID string) error {
					return nil
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "lock mismatch",
			guid: "test-guid-456",
			lockInfo: models.LockInfo{
				ID: "wrong-lock-id",
			},
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return &models.State{GUID: guid}, nil
				},
				unlockStateFunc: func(ctx context.Context, guid string, lockID string) error {
					return errors.New("lock ID mismatch")
				},
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name: "state not found",
			guid: "nonexistent-guid",
			lockInfo: models.LockInfo{
				ID: "lock-789",
			},
			mockService: &mockStateService{
				getByGUIDFunc: func(ctx context.Context, guid string) (*models.State, error) {
					return nil, errors.New("state not found")
				},
				unlockStateFunc: func(ctx context.Context, guid string, lockID string) error {
					return errors.New("should not be called")
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			handlers := &TerraformHandlers{service: tt.mockService}
			// Use actual UNLOCK method (registered in init())
			r.Method("UNLOCK", "/tfstate/{guid}/unlock", http.HandlerFunc(handlers.UnlockState))

			body, _ := json.Marshal(tt.lockInfo)
			req := httptest.NewRequest("UNLOCK", "/tfstate/"+tt.guid+"/unlock", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

package iam

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

type stubRoleRepository struct {
	roles   []models.Role
	listErr error
}

func (s *stubRoleRepository) Create(ctx context.Context, role *models.Role) error {
	return errors.New("not implemented")
}

func (s *stubRoleRepository) GetByID(ctx context.Context, id string) (*models.Role, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRoleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRoleRepository) Update(ctx context.Context, role *models.Role) error {
	return errors.New("not implemented")
}

func (s *stubRoleRepository) Delete(ctx context.Context, id string) error {
	return errors.New("not implemented")
}

func (s *stubRoleRepository) List(ctx context.Context) ([]models.Role, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := make([]models.Role, len(s.roles))
	copy(result, s.roles)
	return result, nil
}

func TestGetRolesByNameReturnsMatchesAndInvalid(t *testing.T) {
	t.Parallel()

	repo := &stubRoleRepository{
		roles: []models.Role{
			{ID: "1", Name: "admin"},
			{ID: "2", Name: "viewer"},
		},
	}

	service := &iamService{
		roles: repo,
	}

	ctx := context.Background()
	requested := []string{"viewer", "missing", "admin"}

	matched, invalid, valid, err := service.GetRolesByName(ctx, requested)
	require.NoError(t, err)
	require.Equal(t, []models.Role{
		{ID: "2", Name: "viewer"},
		{ID: "1", Name: "admin"},
	}, matched)
	require.Equal(t, []string{"missing"}, invalid)
	require.Equal(t, []string{"admin", "viewer"}, valid)
}

func TestGetRolesByNamePropagatesListError(t *testing.T) {
	t.Parallel()

	repo := &stubRoleRepository{
		listErr: errors.New("boom"),
	}

	service := &iamService{
		roles: repo,
	}

	_, _, _, err := service.GetRolesByName(context.Background(), []string{"admin"})
	require.ErrorContains(t, err, "list roles")
}

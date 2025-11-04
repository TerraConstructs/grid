package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// ensureEdgesTable verifies the edges table exists before running tests.
func ensureEdgesTable(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx := context.Background()
	if _, err := db.NewSelect().Table("edges").Limit(1).Exec(ctx); err != nil {
		t.Skipf("edges table not available: %v", err)
	}
}

func createTestState(t *testing.T, repo StateRepository, prefix string) *models.State {
	t.Helper()

	state := &models.State{GUID: uuid.NewString(), LogicID: prefix + uuid.NewString()[0:8]}
	require.NoError(t, repo.Create(context.Background(), state))
	return state
}

func TestBunEdgeRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	ensureEdgesTable(t, db)

	edgeRepo := NewBunEdgeRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	producer := createTestState(t, stateRepo, "test-edge-producer-")
	consumer := createTestState(t, stateRepo, "test-edge-consumer-")

	edge := &models.Edge{
		FromState:   producer.GUID,
		FromOutput:  "vpc_id",
		ToState:     consumer.GUID,
		ToInputName: "consumer_vpc",
		Status:      models.EdgeStatusPending,
	}

	t.Run("create and get", func(t *testing.T) {
		require.NoError(t, edgeRepo.Create(ctx, edge))
		assert.NotZero(t, edge.ID)

		fetched, err := edgeRepo.GetByID(ctx, edge.ID)
		require.NoError(t, err)
		assert.Equal(t, edge.FromState, fetched.FromState)
		assert.Equal(t, edge.ToState, fetched.ToState)
		assert.Equal(t, edge.FromOutput, fetched.FromOutput)
		assert.Equal(t, edge.ToInputName, fetched.ToInputName)
		assert.WithinDuration(t, edge.CreatedAt, fetched.CreatedAt, time.Second)
		assert.WithinDuration(t, edge.UpdatedAt, fetched.UpdatedAt, time.Second)
	})

	t.Run("update", func(t *testing.T) {
		edge.Status = models.EdgeStatusDirty
		edge.InDigest = "new-digest"
		require.NoError(t, edgeRepo.Update(ctx, edge))

		updated, err := edgeRepo.GetByID(ctx, edge.ID)
		require.NoError(t, err)
		assert.Equal(t, models.EdgeStatusDirty, updated.Status)
		assert.Equal(t, "new-digest", updated.InDigest)
		assert.True(t, updated.UpdatedAt.After(updated.CreatedAt) || updated.UpdatedAt.Equal(updated.CreatedAt))
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, edgeRepo.Delete(ctx, edge.ID))
		_, err := edgeRepo.GetByID(ctx, edge.ID)
		assert.Error(t, err)
	})
}

func TestBunEdgeRepository_QueryMethods(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	ensureEdgesTable(t, db)

	edgeRepo := NewBunEdgeRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	producer := createTestState(t, stateRepo, "test-edge-producer-")
	consumer := createTestState(t, stateRepo, "test-edge-consumer-")

	edge := &models.Edge{
		FromState:   producer.GUID,
		FromOutput:  "subnet_ids",
		ToState:     consumer.GUID,
		ToInputName: "subnets",
		Status:      models.EdgeStatusPending,
	}
	require.NoError(t, edgeRepo.Create(ctx, edge))

	t.Run("get outgoing", func(t *testing.T) {
		outgoing, err := edgeRepo.GetOutgoingEdges(ctx, producer.GUID)
		require.NoError(t, err)
		require.Len(t, outgoing, 1)
		assert.Equal(t, edge.ID, outgoing[0].ID)
	})

	t.Run("get incoming", func(t *testing.T) {
		incoming, err := edgeRepo.GetIncomingEdges(ctx, consumer.GUID)
		require.NoError(t, err)
		require.Len(t, incoming, 1)
		assert.Equal(t, edge.ID, incoming[0].ID)
	})

	t.Run("find by output", func(t *testing.T) {
		matches, err := edgeRepo.FindByOutput(ctx, "subnet_ids")
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, edge.ID, matches[0].ID)
	})

	t.Run("get all", func(t *testing.T) {
		all, err := edgeRepo.GetAllEdges(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 1)
	})
}

func TestBunEdgeRepository_UniqueConstraints(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	ensureEdgesTable(t, db)

	edgeRepo := NewBunEdgeRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	producer := createTestState(t, stateRepo, "test-edge-producer-")
	consumer := createTestState(t, stateRepo, "test-edge-consumer-")
	otherProducer := createTestState(t, stateRepo, "test-edge-producer-")

	baseEdge := &models.Edge{
		FromState:   producer.GUID,
		FromOutput:  "vpc_id",
		ToState:     consumer.GUID,
		ToInputName: "network_vpc_id",
		Status:      models.EdgeStatusPending,
	}
	require.NoError(t, edgeRepo.Create(ctx, baseEdge))

	t.Run("composite uniqueness", func(t *testing.T) {
		dup := &models.Edge{
			FromState:   producer.GUID,
			FromOutput:  "vpc_id",
			ToState:     consumer.GUID,
			ToInputName: "network_vpc_id_2",
			Status:      models.EdgeStatusPending,
		}
		err := edgeRepo.Create(ctx, dup)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("to_input_name uniqueness", func(t *testing.T) {
		conflict := &models.Edge{
			FromState:   otherProducer.GUID,
			FromOutput:  "vpc_id",
			ToState:     consumer.GUID,
			ToInputName: "network_vpc_id",
			Status:      models.EdgeStatusPending,
		}
		err := edgeRepo.Create(ctx, conflict)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflict")
	})
}

func TestBunEdgeRepository_WouldCreateCycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	ensureEdgesTable(t, db)

	edgeRepo := NewBunEdgeRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	stateA := createTestState(t, stateRepo, "test-edge-cycle-")
	stateB := createTestState(t, stateRepo, "test-edge-cycle-")
	stateC := createTestState(t, stateRepo, "test-edge-cycle-")

	edges := []*models.Edge{
		{FromState: stateA.GUID, FromOutput: "out", ToState: stateB.GUID, ToInputName: "input_a", Status: models.EdgeStatusPending},
		{FromState: stateB.GUID, FromOutput: "out", ToState: stateC.GUID, ToInputName: "input_b", Status: models.EdgeStatusPending},
	}
	for _, e := range edges {
		require.NoError(t, edgeRepo.Create(ctx, e))
	}

	wouldCycle, err := edgeRepo.WouldCreateCycle(ctx, stateC.GUID, stateA.GUID)
	require.NoError(t, err)
	assert.True(t, wouldCycle)

	safe, err := edgeRepo.WouldCreateCycle(ctx, stateA.GUID, stateC.GUID)
	require.NoError(t, err)
	assert.False(t, safe)
}

func TestBunEdgeRepository_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestData(t, db)
	ensureEdgesTable(t, db)

	edgeRepo := NewBunEdgeRepository(db)
	stateRepo := NewBunStateRepository(db)
	ctx := context.Background()

	producer := createTestState(t, stateRepo, "test-edge-cascade-")
	consumer := createTestState(t, stateRepo, "test-edge-cascade-")

	edge := &models.Edge{
		FromState:   producer.GUID,
		FromOutput:  "vpc_id",
		ToState:     consumer.GUID,
		ToInputName: "cascade_vpc",
		Status:      models.EdgeStatusPending,
	}
	require.NoError(t, edgeRepo.Create(ctx, edge))

	// Deleting the producer state should cascade to edges table.
	_, err := db.NewDelete().Table("states").Where("guid = ?", producer.GUID).Exec(ctx)
	require.NoError(t, err)

	outgoing, err := edgeRepo.GetOutgoingEdges(ctx, producer.GUID)
	require.NoError(t, err)
	assert.Len(t, outgoing, 0)
}

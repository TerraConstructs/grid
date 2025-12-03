package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

// BunEdgeRepository persists dependency edges using Bun ORM against PostgreSQL.
type BunEdgeRepository struct {
	db *bun.DB
}

// NewBunEdgeRepository constructs a repository backed by Bun.
func NewBunEdgeRepository(db *bun.DB) EdgeRepository {
	return &BunEdgeRepository{db: db}
}

// Create inserts a new edge row.
func (r *BunEdgeRepository) Create(ctx context.Context, edge *models.Edge) error {
	if err := edge.ValidateForCreate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	now := time.Now()
	edge.CreatedAt = now
	edge.UpdatedAt = now

	_, err := r.db.NewInsert().Model(edge).Exec(ctx)
	if err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("edge already exists or conflicts with existing edge")
		}
		return fmt.Errorf("insert edge: %w", err)
	}

	return nil
}

// GetByID fetches an edge by its ID.
func (r *BunEdgeRepository) GetByID(ctx context.Context, id int64) (*models.Edge, error) {
	edge := new(models.Edge)
	err := r.db.NewSelect().Model(edge).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("edge with id %d not found", id)
		}
		return nil, fmt.Errorf("query edge: %w", err)
	}

	return edge, nil
}

// Delete removes an edge by its ID.
func (r *BunEdgeRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.NewDelete().Model((*models.Edge)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete edge: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("edge with id %d not found", id)
	}

	return nil
}

// Update persists mutated edge data.
func (r *BunEdgeRepository) Update(ctx context.Context, edge *models.Edge) error {
	edge.UpdatedAt = time.Now()

	result, err := r.db.NewUpdate().
		Model(edge).
		Column("status", "in_digest", "out_digest", "mock_value", "last_in_at", "last_out_at", "updated_at").
		Where("id = ?", edge.ID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("update edge: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("edge with id %d not found", edge.ID)
	}

	return nil
}

// GetOutgoingEdges fetches all edges where the given state is the producer.
func (r *BunEdgeRepository) GetOutgoingEdges(ctx context.Context, fromStateGUID string) ([]models.Edge, error) {
	var edges []models.Edge
	err := r.db.NewSelect().
		Model(&edges).
		Where("from_state = ?", fromStateGUID).
		Order("created_at ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("query outgoing edges: %w", err)
	}

	return edges, nil
}

// GetIncomingEdges fetches all edges where the given state is the consumer.
func (r *BunEdgeRepository) GetIncomingEdges(ctx context.Context, toStateGUID string) ([]models.Edge, error) {
	var edges []models.Edge
	err := r.db.NewSelect().
		Model(&edges).
		Where("to_state = ?", toStateGUID).
		Order("created_at ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("query incoming edges: %w", err)
	}

	return edges, nil
}

// GetAllEdges fetches all edges in the system, ordered by ID (insertion order).
func (r *BunEdgeRepository) GetAllEdges(ctx context.Context) ([]models.Edge, error) {
	var edges []models.Edge
	err := r.db.NewSelect().
		Model(&edges).
		Order("id ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("query all edges: %w", err)
	}

	return edges, nil
}

// FindByOutput finds all edges that reference a specific output key.
func (r *BunEdgeRepository) FindByOutput(ctx context.Context, outputKey string) ([]models.Edge, error) {
	var edges []models.Edge
	err := r.db.NewSelect().
		Model(&edges).
		Where("from_output = ?", outputKey).
		Order("created_at ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("query edges by output: %w", err)
	}

	return edges, nil
}

// GetIncomingEdgesWithProducers fetches incoming edges with producer state data preloaded.
// The FromStateRel field will be populated for each edge.
// This avoids N+1 queries when iterating over edges and accessing producer state.
func (r *BunEdgeRepository) GetIncomingEdgesWithProducers(ctx context.Context, toStateGUID string) ([]*models.Edge, error) {
	var edges []*models.Edge
	err := r.db.NewSelect().
		Model(&edges).
		Relation("FromStateRel"). // Eager load producer states
		Where("to_state = ?", toStateGUID).
		Order("created_at ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("query incoming edges with producers: %w", err)
	}

	return edges, nil
}

// GetOutgoingEdgesWithConsumers fetches outgoing edges with consumer state data preloaded.
// The ToStateRel field will be populated for each edge.
// This avoids N+1 queries when iterating over edges and accessing consumer state.
func (r *BunEdgeRepository) GetOutgoingEdgesWithConsumers(ctx context.Context, fromStateGUID string) ([]*models.Edge, error) {
	var edges []*models.Edge
	err := r.db.NewSelect().
		Model(&edges).
		Relation("ToStateRel"). // Eager load consumer states
		Where("from_state = ?", fromStateGUID).
		Order("created_at ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("query outgoing edges with consumers: %w", err)
	}

	return edges, nil
}

// GetOutgoingEdgesWithValidation fetches outgoing edges with producer output validation status.
// Uses Bun relation to LEFT JOIN state_outputs table for atomic read (single MVCC snapshot).
// The ProducerOutput field will be populated for each edge (nil if output doesn't exist).
func (r *BunEdgeRepository) GetOutgoingEdgesWithValidation(ctx context.Context, fromStateGUID string) ([]EdgeWithValidation, error) {
	var edges []*models.Edge
	err := r.db.NewSelect().
		Model(&edges).
		Relation("ProducerOutput"). // Eager load validation status from state_outputs
		Where("from_state = ?", fromStateGUID).
		Order("created_at ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("query outgoing edges with validation: %w", err)
	}

	// Transform to EdgeWithValidation
	result := make([]EdgeWithValidation, len(edges))
	for i, edge := range edges {
		result[i] = EdgeWithValidation{
			Edge:             *edge,
			ValidationStatus: nil,
			ValidationError:  nil,
		}

		// Extract validation fields if ProducerOutput exists
		if edge.ProducerOutput != nil {
			result[i].ValidationStatus = edge.ProducerOutput.ValidationStatus
			result[i].ValidationError = edge.ProducerOutput.ValidationError
		}
	}

	return result, nil
}

// WouldCreateCycle checks if adding an edge from fromState to toState would create a cycle.
// Uses a recursive CTE to check reachability.
func (r *BunEdgeRepository) WouldCreateCycle(ctx context.Context, fromState, toState string) (bool, error) {
	// Check if toState can already reach fromState
	// If yes, adding fromState -> toState would create a cycle
	var exists bool
	var err error

	// Database-specific query (PostgreSQL uses ::uuid casting, SQLite doesn't)
	dialectName := string(r.db.Dialect().Name())

	if dialectName == "sqlite" {
		// SQLite: WITH RECURSIVE is supported, but without ::uuid casting
		// UUIDs are stored as TEXT in SQLite
		err = r.db.NewRaw(`
			WITH RECURSIVE reachable(node) AS (
				SELECT ? AS node
				UNION ALL
				SELECT e.to_state
				FROM edges e
				JOIN reachable r ON e.from_state = r.node
			)
			SELECT EXISTS(SELECT 1 FROM reachable WHERE node = ?)
		`, toState, fromState).Scan(ctx, &exists)
	} else {
		// PostgreSQL: Use ::uuid type casting
		err = r.db.NewRaw(`
			WITH RECURSIVE reachable(node) AS (
				SELECT ?::uuid AS node
				UNION ALL
				SELECT e.to_state
				FROM edges e
				JOIN reachable r ON e.from_state = r.node
			)
			SELECT EXISTS(SELECT 1 FROM reachable WHERE node = ?::uuid)
		`, toState, fromState).Scan(ctx, &exists)
	}

	if err != nil {
		return false, fmt.Errorf("cycle detection query: %w", err)
	}

	return exists, nil
}

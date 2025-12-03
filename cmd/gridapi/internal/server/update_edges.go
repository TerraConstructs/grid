package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/tfstate"
)

// EdgeUpdateJob manages background edge status updates on tfstate writes
type EdgeUpdateJob struct {
	edgeRepo  repository.EdgeRepository
	stateRepo repository.StateRepository
	locks     sync.Map // map[string]*sync.Mutex keyed by stateGUID
}

// NewEdgeUpdateJob creates a new edge update job manager
func NewEdgeUpdateJob(edgeRepo repository.EdgeRepository, stateRepo repository.StateRepository) *EdgeUpdateJob {
	return &EdgeUpdateJob{
		edgeRepo:  edgeRepo,
		stateRepo: stateRepo,
	}
}

// UpdateEdges processes edge status updates for a state after tfstate write
// This runs asynchronously (best effort, fire-and-forget with per-state mutex)
func (j *EdgeUpdateJob) UpdateEdges(ctx context.Context, stateGUID string, tfstateJSON []byte) {
	// Best effort: parse outputs, log failures internally, do not propagate errors
	outputs, err := tfstate.ParseOutputs(tfstateJSON)
	if err != nil {
		log.Printf("EdgeUpdateJob: failed to parse outputs for state %s: %v", stateGUID, err)
		return
	}

	// Delegate to UpdateEdgesWithOutputs to avoid code duplication
	j.UpdateEdgesWithOutputs(ctx, stateGUID, outputs)
}

// UpdateEdgesWithOutputs processes edge status updates using pre-parsed outputs
// This avoids double-parsing when outputs are already available from state write
func (j *EdgeUpdateJob) UpdateEdgesWithOutputs(ctx context.Context, stateGUID string, outputs map[string]interface{}) {
	// Acquire per-state lock (prevents concurrent updates to same state's edges)
	lockVal, _ := j.locks.LoadOrStore(stateGUID, &sync.Mutex{})
	mu := lockVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Update outgoing edges (this state is producer)
	if err := j.updateOutgoingEdges(ctx, stateGUID, outputs); err != nil {
		log.Printf("EdgeUpdateJob: failed to update outgoing edges for state %s: %v", stateGUID, err)
	}

	// Update incoming edges (this state is consumer, acknowledge observations)
	if err := j.updateIncomingEdges(ctx, stateGUID); err != nil {
		log.Printf("EdgeUpdateJob: failed to update incoming edges for state %s: %v", stateGUID, err)
	}
}

// updateOutgoingEdges updates edges where this state is the producer
func (j *EdgeUpdateJob) updateOutgoingEdges(ctx context.Context, stateGUID string, outputs map[string]interface{}) error {
	edgesWithValidation, err := j.edgeRepo.GetOutgoingEdgesWithValidation(ctx, stateGUID)
	if err != nil {
		return fmt.Errorf("get outgoing edges with validation: %w", err)
	}

	for _, edgeVal := range edgesWithValidation {
		edge := edgeVal.Edge

		// Check if output still exists in tfstate
		outputValue, outputExists := outputs[edge.FromOutput]

		if !outputExists {
			// Output removed from tfstate - mark as missing-output (retain edge)
			if edge.Status != models.EdgeStatusMissingOutput {
				edge.Status = models.EdgeStatusMissingOutput
				if err := j.edgeRepo.Update(ctx, &edge); err != nil {
					log.Printf("EdgeUpdateJob: failed to mark edge %d as missing-output: %v", edge.ID, err)
				}
			}
			continue
		}

		// Compute new fingerprint
		newDigest := tfstate.ComputeFingerprint(outputValue)
		if newDigest == "" {
			continue // Skip if fingerprint computation failed
		}

		// Check if this is a mock edge transitioning to real output
		if edge.Status == models.EdgeStatusMock {
			// Transition from mock to real output
			edge.Status = models.EdgeStatusPending
			edge.MockValue = nil
			edge.InDigest = newDigest
			now := time.Now()
			edge.LastInAt = &now

			if err := j.edgeRepo.Update(ctx, &edge); err != nil {
				log.Printf("EdgeUpdateJob: failed to transition mock edge %d: %v", edge.ID, err)
			}
			continue
		}

		// Compute new status using composite model (drift × validation)
		newStatus := deriveEdgeStatusWithValidation(newDigest, edge.OutDigest, edgeVal.ValidationStatus, true)

		// Check if producer output changed OR validation status changed
		digestChanged := (edge.InDigest != newDigest)
		statusChanged := (edge.Status != newStatus)

		if digestChanged || statusChanged {
			// Update digest if it changed
			if digestChanged {
				edge.InDigest = newDigest
				now := time.Now()
				edge.LastInAt = &now
			}

			// Always update status to new computed value
			edge.Status = newStatus

			if err := j.edgeRepo.Update(ctx, &edge); err != nil {
				log.Printf("EdgeUpdateJob: failed to update edge %d: %v", edge.ID, err)
			}
		}
	}

	return nil
}

// updateIncomingEdges updates edges where this state is the consumer
func (j *EdgeUpdateJob) updateIncomingEdges(ctx context.Context, stateGUID string) error {
	incomingEdges, err := j.edgeRepo.GetIncomingEdges(ctx, stateGUID)
	if err != nil {
		return fmt.Errorf("get incoming edges: %w", err)
	}

	for _, edge := range incomingEdges {
		// Check if consumer has observed the current producer output
		if edge.InDigest != "" && edge.OutDigest != edge.InDigest {
			// Consumer is observing - update out_digest to match in_digest
			edge.OutDigest = edge.InDigest
			now := time.Now()
			edge.LastOutAt = &now

			// Transition to clean status, preserving validation dimension
			// dirty-invalid → clean-invalid, dirty → clean
			if edge.Status == models.EdgeStatusDirtyInvalid {
				edge.Status = models.EdgeStatusCleanInvalid
			} else {
				edge.Status = models.EdgeStatusClean
			}

			if err := j.edgeRepo.Update(ctx, &edge); err != nil {
				log.Printf("EdgeUpdateJob: failed to update edge %d observation: %v", edge.ID, err)
			}
		}
	}

	return nil
}

// deriveEdgeStatusWithValidation computes edge status using composite model.
// Combines two orthogonal dimensions: drift (in_digest vs out_digest) and validation (schema compliance).
// Parameters:
//   - inDigest: producer's current output fingerprint
//   - outDigest: consumer's observed fingerprint
//   - validationStatus: validation_status from state_outputs (nil if no schema or not validated)
//   - outputExists: whether the output key exists in producer's tfstate
//
// Returns:
//   - missing-output: if output doesn't exist (highest priority)
//   - clean: in_digest == out_digest AND (valid OR no schema)
//   - clean-invalid: in_digest == out_digest AND invalid
//   - dirty: in_digest != out_digest AND (valid OR no schema)
//   - dirty-invalid: in_digest != out_digest AND invalid
//   - pending: no in_digest yet
func deriveEdgeStatusWithValidation(inDigest, outDigest string, validationStatus *string, outputExists bool) models.EdgeStatus {
	// Priority 1: Output existence (overrides everything)
	if !outputExists {
		return models.EdgeStatusMissingOutput
	}

	// Priority 2: Pending state (no producer data yet)
	if inDigest == "" {
		return models.EdgeStatusPending
	}

	// Compute drift dimension
	isDirty := (outDigest == "" || inDigest != outDigest)

	// Compute validation dimension
	isInvalid := (validationStatus != nil && *validationStatus == "invalid")

	// Composite matrix: drift × validation
	if isDirty && isInvalid {
		return models.EdgeStatusDirtyInvalid
	}
	if isDirty {
		return models.EdgeStatusDirty
	}
	if isInvalid {
		return models.EdgeStatusCleanInvalid
	}
	return models.EdgeStatusClean
}

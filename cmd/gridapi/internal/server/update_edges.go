package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/tfstate"
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
	// Acquire per-state lock (prevents concurrent updates to same state's edges)
	lockVal, _ := j.locks.LoadOrStore(stateGUID, &sync.Mutex{})
	mu := lockVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Best effort: parse outputs, log failures internally, do not propagate errors
	outputs, err := tfstate.ParseOutputs(tfstateJSON)
	if err != nil {
		log.Printf("EdgeUpdateJob: failed to parse outputs for state %s: %v", stateGUID, err)
		return
	}

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
	outgoingEdges, err := j.edgeRepo.GetOutgoingEdges(ctx, stateGUID)
	if err != nil {
		return fmt.Errorf("get outgoing edges: %w", err)
	}

	for _, edge := range outgoingEdges {
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

		// Check if producer output changed
		if edge.InDigest != newDigest {
			edge.InDigest = newDigest
			now := time.Now()
			edge.LastInAt = &now

			// Recompute status vs consumer observation
			edge.Status = deriveEdgeStatus(newDigest, edge.OutDigest)

			if err := j.edgeRepo.Update(ctx, &edge); err != nil {
				log.Printf("EdgeUpdateJob: failed to update edge %d digest: %v", edge.ID, err)
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
			edge.Status = models.EdgeStatusClean

			if err := j.edgeRepo.Update(ctx, &edge); err != nil {
				log.Printf("EdgeUpdateJob: failed to update edge %d observation: %v", edge.ID, err)
			}
		}
	}

	return nil
}

// deriveEdgeStatus computes edge status based on in_digest and out_digest
func deriveEdgeStatus(inDigest, outDigest string) models.EdgeStatus {
	if inDigest == "" {
		return models.EdgeStatusPending
	}
	if outDigest == "" {
		return models.EdgeStatusDirty
	}
	if inDigest == outDigest {
		return models.EdgeStatusClean
	}
	return models.EdgeStatusDirty
}

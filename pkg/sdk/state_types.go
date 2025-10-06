package sdk

import (
	"time"

	statev1 "github.com/terraconstructs/grid/api/state/v1"
)

// StateReference identifies a state by logic ID or GUID.
type StateReference struct {
	GUID    string
	LogicID string
}

// BackendConfig represents Terraform HTTP backend endpoints for a state.
type BackendConfig struct {
	Address       string
	LockAddress   string
	UnlockAddress string
}

// State describes a Terraform state along with its backend configuration.
type State struct {
	GUID          string
	LogicID       string
	BackendConfig BackendConfig
}

// StateSummary conveys lightweight information about a state returned by ListStates.
type StateSummary struct {
	GUID               string
	LogicID            string
	Locked             bool
	SizeBytes          int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ComputedStatus     string
	DependencyLogicIDs []string
}

// LockInfo contains details about a Terraform state lock.
type LockInfo struct {
	ID        string
	Operation string
	Info      string
	Who       string
	Version   string
	Created   *time.Time
	Path      string
}

// StateLock reports whether a state is locked along with lock metadata when present.
type StateLock struct {
	Locked bool
	Info   *LockInfo
}

// DependencyEdge represents a dependency relationship between producer and consumer states.
type DependencyEdge struct {
	ID             int64
	From           StateReference
	FromOutput     string
	To             StateReference
	ToInputName    string
	Status         string
	InDigest       string
	OutDigest      string
	MockValueJSON  string
	LastProducedAt *time.Time
	LastConsumedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ProducerState describes a unique producer within a dependency graph.
type ProducerState struct {
	State         StateReference
	BackendConfig BackendConfig
}

// DependencyGraph captures the full dependency topology for a consumer state.
type DependencyGraph struct {
	Consumer  StateReference
	Producers []ProducerState
	Edges     []DependencyEdge
}

// TopologyLayer represents a level within a topological ordering of states.
type TopologyLayer struct {
	Level  int
	States []StateReference
}

// StatusSummary aggregates incoming edge statuses for a state.
type StatusSummary struct {
	IncomingClean   int
	IncomingDirty   int
	IncomingPending int
	IncomingUnknown int
}

// IncomingEdge conveys details about an incoming producer edge when computing status.
type IncomingEdge struct {
	ID             int64
	From           StateReference
	FromOutput     string
	Status         string
	InDigest       string
	OutDigest      string
	LastProducedAt *time.Time
	LastConsumedAt *time.Time
}

// StateStatus captures runtime dependency status information for a state.
type StateStatus struct {
	State    StateReference
	Status   string
	Incoming []IncomingEdge
	Summary  StatusSummary
}

// AddDependencyInput describes the parameters used to create a dependency edge.
type AddDependencyInput struct {
	From          StateReference
	FromOutput    string
	To            StateReference
	ToInputName   string
	MockValueJSON string
}

// AddDependencyResult returns the created or existing dependency edge and metadata.
type AddDependencyResult struct {
	Edge          DependencyEdge
	AlreadyExists bool
}

// CreateStateInput describes the payload required to create a Terraform state.
type CreateStateInput struct {
	GUID    string
	LogicID string
}

// TopologyDirection indicates the traversal direction for topological ordering.
type TopologyDirection string

const (
	// Downstream traverses from producer to dependents (default).
	Downstream TopologyDirection = "downstream"
	// Upstream traverses from consumer to producers.
	Upstream TopologyDirection = "upstream"
)

// TopologyInput configures a topological ordering request.
type TopologyInput struct {
	Root      StateReference
	Direction TopologyDirection
}

// OutputKey represents a Terraform output name and metadata.
type OutputKey struct {
	Key       string
	Sensitive bool
}

// StateInfo provides comprehensive information about a state including dependencies, dependents, and outputs.
type StateInfo struct {
	State         StateReference
	BackendConfig BackendConfig
	Dependencies  []DependencyEdge
	Dependents    []DependencyEdge
	Outputs       []OutputKey
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// helper conversions from proto messages ----------------------------------------------------

func backendConfigFromProto(pb *statev1.BackendConfig) BackendConfig {
	if pb == nil {
		return BackendConfig{}
	}
	return BackendConfig{
		Address:       pb.Address,
		LockAddress:   pb.LockAddress,
		UnlockAddress: pb.UnlockAddress,
	}
}

func stateSummaryFromProto(info *statev1.StateInfo) StateSummary {
	if info == nil {
		return StateSummary{}
	}
	summary := StateSummary{
		GUID:               info.GetGuid(),
		LogicID:            info.GetLogicId(),
		Locked:             info.GetLocked(),
		SizeBytes:          info.GetSizeBytes(),
		DependencyLogicIDs: append([]string(nil), info.DependencyLogicIds...),
	}
	if info.CreatedAt != nil {
		t := info.CreatedAt.AsTime()
		summary.CreatedAt = t
	}
	if info.UpdatedAt != nil {
		t := info.UpdatedAt.AsTime()
		summary.UpdatedAt = t
	}
	if info.ComputedStatus != nil {
		summary.ComputedStatus = info.GetComputedStatus()
	}
	return summary
}

func lockInfoFromProto(lock *statev1.LockInfo) *LockInfo {
	if lock == nil {
		return nil
	}
	var created *time.Time
	if lock.Created != nil {
		t := lock.Created.AsTime()
		created = &t
	}
	return &LockInfo{
		ID:        lock.GetId(),
		Operation: lock.GetOperation(),
		Info:      lock.GetInfo(),
		Who:       lock.GetWho(),
		Version:   lock.GetVersion(),
		Created:   created,
		Path:      lock.GetPath(),
	}
}

func stateLockFromProto(lock *statev1.StateLock) StateLock {
	if lock == nil {
		return StateLock{}
	}
	return StateLock{
		Locked: lock.GetLocked(),
		Info:   lockInfoFromProto(lock.Info),
	}
}

func outputKeyFromProto(pb *statev1.OutputKey) OutputKey {
	if pb == nil {
		return OutputKey{}
	}
	return OutputKey{
		Key:       pb.GetKey(),
		Sensitive: pb.GetSensitive(),
	}
}

func outputKeysFromProto(pbs []*statev1.OutputKey) []OutputKey {
	if pbs == nil {
		return []OutputKey{}
	}
	outputs := make([]OutputKey, 0, len(pbs))
	for _, pb := range pbs {
		outputs = append(outputs, outputKeyFromProto(pb))
	}
	return outputs
}

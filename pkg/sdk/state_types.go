package sdk

import statev1 "github.com/terraconstructs/grid/api/state/v1"

// BackendConfig aliases the generated protobuf Terraform backend configuration message.
type BackendConfig = statev1.BackendConfig

// StateSummary exposes the generated StateInfo message for SDK consumers.
type StateSummary = statev1.StateInfo

// StateLock mirrors the generated StateLock message, including optional lock metadata.
type StateLock = statev1.StateLock

// LockInfo references the generated lock payload structure from the API contract.
type LockInfo = statev1.LockInfo

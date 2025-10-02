package server

import "errors"

var (
	// ErrLogicIDRequired is returned when logic_id is missing
	ErrLogicIDRequired = errors.New("logic_id is required")

	// ErrStateNotFound is returned when a state does not exist
	ErrStateNotFound = errors.New("state not found")

	// ErrStateLocked is returned when attempting to modify a locked state
	ErrStateLocked = errors.New("state is locked")

	// ErrLockConflict is returned when attempting to lock an already-locked state
	ErrLockConflict = errors.New("state is already locked")
)

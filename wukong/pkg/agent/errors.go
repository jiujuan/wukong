package agent

import "errors"

// ErrAgentNotFound is returned when a run cannot be routed to an agent profile.
var ErrAgentNotFound = errors.New("agent not found")

// ErrRuntimeNotStarted is returned when a run is requested before Start.
var ErrRuntimeNotStarted = errors.New("agent runtime not started")

// ErrLoopPaused is returned when a loop pauses and needs resume input.
var ErrLoopPaused = errors.New("agent loop paused")

// ErrCheckpointNotFound is returned when resume cannot find the requested checkpoint.
var ErrCheckpointNotFound = errors.New("agent checkpoint not found")

// ErrActionExecutorNotFound is returned when no executor can handle a planned action.
var ErrActionExecutorNotFound = errors.New("agent action executor not found")

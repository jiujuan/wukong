package sandbox

import "errors"

var (
	ErrOutputLimitExceeded = errors.New("sandbox output limit exceeded")
	ErrInvalidRequest      = errors.New("sandbox invalid request")
	ErrRunnerNotFound      = errors.New("sandbox runner not found")
	ErrCommandNotAllowed   = errors.New("sandbox command not allowed")
	ErrWorkDirNotAllowed   = errors.New("sandbox workdir not allowed")
)

package process

import (
	"time"
)

const contextTimeout = 10 * time.Second

// Process is an arbitrary process that can be started or stopped
type Process interface {
	Start() error
	Stop() error
	IsRunning() (bool, error)
}

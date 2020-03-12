package process

import (
	"time"
)

const contextTimeout = 10 * time.Second

// Process is an arbitrary process that can be started or stopped
type Process interface {
	Start() error
	Stop() error
	IsRunning() bool
}

// CancelFunc will cancel a timeout of a process
type CancelFunc func()

// Timeout stops a process after a specific duration
func Timeout(proc Process, timeout time.Duration) CancelFunc {
	if !proc.IsRunning() {
		return func() {}
	}

	cancel := make(chan bool, 1)

	select {
	case <-cancel:
	case <-time.After(timeout):
		proc.Stop()
	}

	return func() { cancel <- true }
}

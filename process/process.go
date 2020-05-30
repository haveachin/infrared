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

func New(cfg Config) (Process, error) {
	if cfg.hasPortainerConfig() {
		return NewPortainer(
				cfg.ContainerName,
				cfg.Portainer.Address,
				cfg.Portainer.EndpointID,
				cfg.Portainer.Username,
				cfg.Portainer.Password,
			)
	}

	return NewDocker(cfg.ContainerName)
}

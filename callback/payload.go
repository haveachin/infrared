package callback

import "time"

type Payload struct {
	Timestamp           time.Time `json:"timestamp"`
	Action              string    `json:"action"`
	Success             bool      `json:"success"`
	Message             string    `json:"message"`
	ContainerName       string    `json:"containerName"`
	PortainerEndpointID string    `json:"portainerEndpointId,omitempty"`
}

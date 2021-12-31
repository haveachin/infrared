package infrared

import (
	"net"

	"github.com/go-logr/logr"
)

type Gateway interface {
	// GetID resturns the ID of the gateway
	GetID() string
	// GetServerIDs returns the IDs of the servers
	// that are registered in that gateway
	GetServerIDs() []string
	GetServerNotFoundMessage() string
	SetLogger(log logr.Logger)
	ListenAndServe(cpnChan chan<- net.Conn) error
}

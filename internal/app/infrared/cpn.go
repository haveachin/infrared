package infrared

import (
	"errors"
	"net"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
)

// Processing Node
type CPN struct {
	ConnProcessor
	Log logr.Logger
}

type ConnProcessor interface {
	ProcessConn(c net.Conn) (ProcessedConn, error)
	GetClientTimeout() time.Duration
}

func (cpn *CPN) Start(cpnChan <-chan net.Conn, srvChan chan<- ProcessedConn) {
	for {
		c, ok := <-cpnChan
		if !ok {
			break
		}

		keysAndValues := []interface{}{
			"network", c.LocalAddr().Network(),
			"localAddr", c.LocalAddr(),
			"remoteAddr", c.RemoteAddr(),
		}
		cpn.Log.Info("starting to process connection", keysAndValues...)
		event.Push(PreConnProcessingEventTopic, keysAndValues...)

		c.SetDeadline(time.Now().Add(cpn.GetClientTimeout()))
		pc, err := cpn.ProcessConn(c)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				cpn.Log.Info("disconnecting connection; exceeded processing deadline", keysAndValues...)
			} else {
				cpn.Log.Error(err, "disconnecting connection; processing failed", keysAndValues...)
			}
			c.Close()
			continue
		}
		c.SetDeadline(time.Time{})

		keysAndValues = append(keysAndValues,
			"serverAddr", pc.ServerAddr(),
			"username", pc.Username(),
			"gatewayId", pc.GatewayID(),
		)
		cpn.Log.Info("sending client to server gateway", keysAndValues...)
		event.Push(PostConnProcessingEventTopic, keysAndValues...)

		srvChan <- pc
	}
}

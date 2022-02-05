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
			"localAddress", c.LocalAddr(),
			"remoteAddress", c.RemoteAddr(),
		}
		cpn.Log.Info("processing connection", keysAndValues...)
		event.Push(ConnProcessingEventTopic, keysAndValues...)

		c.SetDeadline(time.Now().Add(cpn.GetClientTimeout()))
		pc, err := cpn.ProcessConn(c)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				cpn.Log.Info("client exceeded processing deadline", keysAndValues...)
			} else {
				cpn.Log.Error(err, "processing", keysAndValues...)
			}
			c.Close()
			continue
		}
		c.SetDeadline(time.Time{})
		srvChan <- pc
	}
}

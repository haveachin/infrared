package infrared

import (
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

type ConnPool struct {
	Logger *zap.Logger
}

func (cp *ConnPool) Start(poolChan <-chan ConnTunnel) {
	for {
		ct, ok := <-poolChan
		if !ok {
			break
		}

		cp.Logger.Info("starting tunnel", logConn(ct.Conn)...)

		go func() {
			ct.Start()
			cp.Logger.Info("disconnecting client", logConn(ct.Conn)...)
			event.Push(ClientLeaveEventTopic, ct.Metadata)
		}()
	}
}

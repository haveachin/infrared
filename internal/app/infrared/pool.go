package infrared

import (
	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
)

type ConnPool struct {
	Log logr.Logger
}

func (cp *ConnPool) Start(poolChan <-chan ConnTunnel) {
	for {
		ct, ok := <-poolChan
		if !ok {
			break
		}

		cp.Log.Info("starting tunnel", ct.Metadata...)

		go func() {
			ct.Start()
			cp.Log.Info("disconnecting client", ct.Metadata...)
			event.Push(ClientLeaveEventTopic, ct.Metadata)
		}()
	}
}

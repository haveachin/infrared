package infrared

import (
	"github.com/go-logr/logr"
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

		cp.Log.Info("starting tunnel",
			"client", ct.Conn.RemoteAddr(),
			"server", ct.RemoteConn.RemoteAddr(),
		)

		go ct.Start()
	}
}

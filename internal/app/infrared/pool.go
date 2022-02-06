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

		keysAndValues := []interface{}{
			"network", ct.Conn.LocalAddr().Network(),
			"localAddr", ct.Conn.LocalAddr().String(),
			"remoteAddr", ct.Conn.RemoteAddr().String(),
			"serverAddr", ct.Conn.ServerAddr(),
			"username", ct.Conn.Username(),
			"gatewayId", ct.Conn.GatewayID(),
			"serverLocalAddr", ct.RemoteConn.LocalAddr().String(),
			"serverRemoteAddr", ct.RemoteConn.RemoteAddr().String(),
			"serverWebhookIds", ct.WebhookIds,
		}

		cp.Log.Info("starting tunnel", keysAndValues...)

		go func() {
			ct.Start()
			cp.Log.Info("disconnecting client", keysAndValues...)
			event.Push(ClientLeaveEventTopic, keysAndValues...)
		}()
	}
}

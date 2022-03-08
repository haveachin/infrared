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

		connLogger := cp.Logger.With(logProcessedConn(ct.Conn)...)
		connLogger.Info("starting server processing")

		go func(logger *zap.Logger) {
			if ct.Conn.IsLoginRequest() {
				event.Push(ClientJoinEventTopic, nil)
			}

			if err := ct.ProcessConn(); err != nil {
				logger.Debug("failed to process client", zap.Error(err))
				return
			}

			logger.Info("disconnecting client")
			event.Push(ClientLeaveEventTopic, nil)
		}(connLogger)
	}
}

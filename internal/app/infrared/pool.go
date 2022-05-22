package infrared

import (
	"errors"
	"sync"

	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

type ConnPoolConfig struct {
	In     <-chan ConnTunnel
	Logger *zap.Logger
}

type ConnPool struct {
	ConnPoolConfig

	reload chan func()
	quit   chan bool
	mu     sync.Mutex
	pool   []ConnTunnel
}

func (cp *ConnPool) Start() {
	cp.reload = make(chan func())
	cp.quit = make(chan bool)

	for {
		select {
		case ct, ok := <-cp.In:
			if !ok {
				break
			}

			cp.addToPool(ct)

			go func(logger *zap.Logger) {
				if ct.Conn.IsLoginRequest() {
					event.Push(PlayerJoinEventTopic, PlayerJoinEvent{
						ProcessedConn: ct.Conn,
						Server:        ct.Server,
					})
				}

				logger.Info("connecting client to server")
				if err := ct.Start(); err != nil {
					logger.Info("closing connection", zap.Error(err))
					return
				}
				ct.Conn.Close()

				logger.Info("disconnecting client")
				event.Push(PlayerLeaveEventTopic, PlayerLeaveEvent{
					ProcessedConn: ct.Conn,
					Server:        ct.Server,
				})
				cp.removeFromPool(ct)
			}(cp.Logger.With(logProcessedConn(ct.Conn)...))
		case reload := <-cp.reload:
			reload()
		case <-cp.quit:
			return
		}
	}
}

func (cp *ConnPool) Reload(cfg ConnPoolConfig) {
	cp.reload <- func() {
		cp.ConnPoolConfig = cfg
	}
}

func (cp *ConnPool) Close() error {
	if cp.quit == nil {
		return errors.New("server gateway was not running")
	}

	cp.quit <- true
	return nil
}

func (cp *ConnPool) addToPool(ct ConnTunnel) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.pool = append(cp.pool, ct)
}

func (cp *ConnPool) removeFromPool(ct ConnTunnel) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.pool = append(cp.pool, ct)
}

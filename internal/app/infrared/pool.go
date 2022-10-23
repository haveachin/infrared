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
	cp.pool = make([]ConnTunnel, 0, 100)

	for {
		select {
		case ct, ok := <-cp.In:
			if !ok {
				break
			}

			if ct.Conn.IsLoginRequest() {
				go cp.handlePlayerLogin(ct)
			} else {
				go cp.handlePlayerStatus(ct)
			}

		case reload := <-cp.reload:
			reload()
		case <-cp.quit:
			return
		}
	}
}

func (cp *ConnPool) handlePlayerStatus(ct ConnTunnel) {
	defer ct.Conn.Close()

	logger := cp.Logger.With(logProcessedConn(ct.Conn)...)
	logger.Info("connecting client to server")
	if err := ct.Start(); err != nil {
		logger.Info("closing connection", zap.Error(err))
		return
	}
}

func (cp *ConnPool) handlePlayerLogin(ct ConnTunnel) {
	defer ct.Conn.Close()

	i := cp.addToPool(ct)
	defer cp.removeFromPool(i)

	logger := cp.Logger.With(logProcessedConn(ct.Conn)...)
	event.Push(PlayerJoinEvent{
		ProcessedConn: ct.Conn,
		Server:        ct.Server,
		MatchedDomain: ct.MatchedDomain,
	}, PlayerJoinEventTopic)

	logger.Info("connecting client to server")
	if err := ct.Start(); err != nil {
		logger.Info("closing connection", zap.Error(err))
		return
	}

	logger.Info("disconnecting client")
	event.Push(PlayerLeaveEvent{
		ProcessedConn: ct.Conn,
		Server:        ct.Server,
		MatchedDomain: ct.MatchedDomain,
	}, PlayerLeaveEventTopic)
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

func (cp *ConnPool) addToPool(ct ConnTunnel) int {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.pool = append(cp.pool, ct)
	return len(cp.pool) - 1
}

func (cp *ConnPool) removeFromPool(index int) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	n := len(cp.pool) - 1
	cp.pool[index] = cp.pool[n]
	cp.pool = cp.pool[:n]
}

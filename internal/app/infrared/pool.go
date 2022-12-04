package infrared

import (
	"errors"
	"net"
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
	pool   map[net.Addr]ConnTunnel
}

func (cp *ConnPool) Start() {
	cp.reload = make(chan func())
	cp.quit = make(chan bool)
	cp.pool = map[net.Addr]ConnTunnel{}

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
	if _, err := ct.Start(); err != nil {
		logger.Info("closing connection", zap.Error(err))
		return
	}
}

func (cp *ConnPool) handlePlayerLogin(ct ConnTunnel) {
	defer ct.Conn.Close()

	cp.addToPool(ct)
	defer cp.removeFromPool(ct)

	logger := cp.Logger.With(logProcessedConn(ct.Conn)...)
	replyChan := event.Request(PlayerJoinEvent{
		ProcessedConn: ct.Conn,
		Server:        ct.Server,
		MatchedDomain: ct.MatchedDomain,
	}, PlayerJoinEventTopic)

	if isEventCanceled(replyChan, logger) {
		return
	}

	logger.Info("connecting client to server")
	consumedBytes, err := ct.Start()
	if err != nil {
		logger.Info("closing connection", zap.Error(err))
		return
	}

	logger.Info("disconnecting client")
	event.Push(PlayerLeaveEvent{
		ProcessedConn: ct.Conn,
		Server:        ct.Server,
		MatchedDomain: ct.MatchedDomain,
		ConsumedBytes: consumedBytes,
	}, PlayerLeaveEventTopicAsync)
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
	cp.pool[ct.Conn.RemoteAddr()] = ct
}

func (cp *ConnPool) removeFromPool(ct ConnTunnel) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	delete(cp.pool, ct.Conn.RemoteAddr())
}

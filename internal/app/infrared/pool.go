package infrared

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

type ConnPoolConfig struct {
	In       <-chan ConnTunnel
	Logger   *zap.Logger
	EventBus event.Bus
}

type ConnPool struct {
	ConnPoolConfig

	mu     sync.Mutex
	reload chan func()
	quit   chan bool
	pool   map[net.Addr]ConnTunnel
}

func (cp *ConnPool) Start() {
	cp.mu.Lock()
	cp.reload = make(chan func())
	cp.quit = make(chan bool)
	cp.mu.Unlock()
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
	logger.Debug("connecting client to server")
	ct.Conn.SetDeadline(time.Now().Add(time.Millisecond * 500))
	if _, err := ct.Start(); err != nil {
		logger.Debug("closing connection", zap.Error(err))
		return
	}
}

func (cp *ConnPool) handlePlayerLogin(ct ConnTunnel) {
	defer ct.Conn.Close()

	logger := cp.Logger.With(logProcessedConn(ct.Conn)...)
	consumedBytes := int64(0)

	cp.addToPool(ct)
	defer func() {
		logger.Debug("disconnected client")
		cp.removeFromPool(ct)
		cp.EventBus.Push(PlayerLeaveEvent{
			Player:        ct.Conn,
			Server:        ct.Server,
			MatchedDomain: ct.MatchedDomain,
			ConsumedBytes: consumedBytes,
		}, PlayerLeaveEventTopicAsync)
	}()

	replyChan := cp.EventBus.Request(PlayerJoinEvent{
		Player:        ct.Conn,
		Server:        ct.Server,
		MatchedDomain: ct.MatchedDomain,
	}, PlayerJoinEventTopic)
	if isEventCanceled(replyChan, logger) {
		return
	}

	logger.Info("connecting player to server")
	var err error
	consumedBytes, err = ct.Start()
	if err != nil {
		logger.Info("closing connection", zap.Error(err))
	}
}

func (cp *ConnPool) Reload(cfg ConnPoolConfig) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.reload == nil {
		return
	}

	cp.reload <- func() {
		cp.ConnPoolConfig = cfg
	}
}

func (cp *ConnPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

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

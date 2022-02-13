package infrared

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
)

var (
	ErrIsAlreadyRunning = errors.New("is already running")
	ErrChannelClosed    = errors.New("channel closed")
)

type ConnProcessor interface {
	ProcessConn(c net.Conn) (ProcessedConn, error)
	GetClientTimeout() time.Duration
}

// CPN is a connection processing node
type CPN struct {
	ConnProcessor
	In       <-chan net.Conn
	Out      chan<- ProcessedConn
	Log      logr.Logger
	EventBus event.Bus

	errorLocker
	ctx context.Context

	mu     sync.Mutex
	cancel context.CancelFunc
}

func (cpn *CPN) ListenAndServe() error {
	if err := cpn.lock(); err != nil {
		return err
	}
	defer cpn.unlock()

	cpn.mu.Lock()
	cpn.ctx, cpn.cancel = context.WithCancel(context.Background())
	cpn.mu.Unlock()

	for {
		select {
		case <-cpn.ctx.Done():
			return nil
		case c, ok := <-cpn.In:
			if !ok {
				return ErrChannelClosed
			}

			keysAndValues := []interface{}{
				"network", c.LocalAddr().Network(),
				"localAddr", c.LocalAddr().String(),
				"remoteAddr", c.RemoteAddr().String(),
			}
			cpn.Log.Info("starting to process connection", keysAndValues...)
			cpn.EventBus.Push(PreConnProcessingEventTopic, keysAndValues)

			c.SetDeadline(time.Now().Add(cpn.GetClientTimeout()))
			pc, err := cpn.ConnProcessor.ProcessConn(c)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					cpn.Log.Info("disconnecting connection; exceeded processing deadline", keysAndValues...)
				} else {
					cpn.Log.Error(err, "disconnecting connection; processing failed", keysAndValues...)
				}
				c.Close()
				continue
			}
			c.SetDeadline(time.Time{})

			keysAndValues = append(keysAndValues,
				"serverAddr", pc.ServerAddr(),
				"username", pc.Username(),
				"gatewayId", pc.GatewayID(),
				"isLoginRequest", pc.IsLoginRequest(),
			)
			cpn.Log.Info("sending client to server gateway", keysAndValues...)
			cpn.EventBus.Push(PostConnProcessingEventTopic, keysAndValues)

			cpn.Out <- pc
		}
	}
}

func (cpn *CPN) Close() {
	cpn.mu.Lock()
	defer cpn.mu.Unlock()

	if cpn.cancel == nil {
		return
	}

	cpn.cancel()
}

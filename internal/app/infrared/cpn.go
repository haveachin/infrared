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

type ConnProcessor interface {
	ProcessConn(c net.Conn) (ProcessedConn, error)
	ClientTimeout() time.Duration
}

// CPN is a connection processing node
type CPN struct {
	ConnProcessor
	In       <-chan net.Conn
	Out      chan<- ProcessedConn
	Log      logr.Logger
	EventBus event.Bus
}

func (cpn CPN) ListenAndServe(ctx context.Context) {
	for c := range cpn.In {
		keysAndValues := []interface{}{
			"network", c.LocalAddr().Network(),
			"localAddr", c.LocalAddr().String(),
			"remoteAddr", c.RemoteAddr().String(),
		}
		cpn.Log.Info("starting to process connection", keysAndValues...)
		cpn.EventBus.Push(PreConnProcessingEventTopic, keysAndValues)

		c.SetDeadline(time.Now().Add(cpn.ClientTimeout()))
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

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

type CPNPool struct {
	CPN CPN

	mu  sync.Mutex
	cfs []context.CancelFunc
}

func (p *CPNPool) Start(n int) {
	ctx, cancel := context.WithCancel(context.Background())
	p.CPN.ListenAndServe(ctx)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cfs == nil {
		p.cfs = make([]context.CancelFunc, n)
	}

	p.cfs = append(p.cfs, cancel)
}

func (p *CPNPool) Stop(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cfs == nil || len(p.cfs) == 0 {
		return
	}

	l := len(p.cfs)
	if n > l {
		n = l
	}
	size := l - n

	for ; n > size; n-- {
		p.cfs[n]()
	}

	p.cfs = append(p.cfs, p.cfs[:size]...)
}

func (p *CPNPool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cfs == nil {
		return 0
	}

	return len(p.cfs)
}

func (p *CPNPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, cf := range p.cfs {
		cf()
	}
}

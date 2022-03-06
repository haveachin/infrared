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

// ConnProcessor represents a
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
	for {
		select {
		case c, ok := <-cpn.In:
			if !ok {
				return
			}

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
		case <-ctx.Done():
			return
		}
	}
}

type CPNPool struct {
	CPN CPN

	mu  sync.Mutex
	cfs []context.CancelFunc
}

func (p *CPNPool) SetSize(n int) {
	if n < 0 {
		n = 0
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	len := len(p.cfs)
	if len > n {
		p.remove(len - n)
	} else if len < n {
		p.add(n - len)
	}
}

func (p *CPNPool) add(n int) {
	cfs := make([]context.CancelFunc, n)
	for i := 0; i < n; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		go p.CPN.ListenAndServe(ctx)
		cfs[i] = cancel
	}

	if p.cfs == nil {
		p.cfs = cfs
	} else {
		p.cfs = append(p.cfs, cfs...)
	}
}

func (p *CPNPool) remove(n int) {
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
	p.cfs = nil
}

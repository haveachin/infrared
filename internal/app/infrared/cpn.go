package infrared

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

// ConnProcessor represents a
type ConnProcessor interface {
	ProcessConn(c net.Conn) (net.Conn, error)
	ClientTimeout() time.Duration
}

// CPN is a connection processing node
type CPN struct {
	ConnProcessor
	In       <-chan Conn
	Out      chan<- ProcessedConn
	Logger   *zap.Logger
	EventBus event.Bus
}

func (cpn CPN) ListenAndServe(ctx context.Context) {
	for {
		select {
		case c, ok := <-cpn.In:
			if !ok {
				return
			}

			connLogger := cpn.Logger.With(logConn(c)...)
			connLogger.Debug("starting to process connection")
			cpn.EventBus.Push(PreConnProcessingEventTopic, PreConnProcessingEvent{
				Conn: c,
			})

			c.SetDeadline(time.Now().Add(cpn.ClientTimeout()))
			pc, err := cpn.ConnProcessor.ProcessConn(c)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					connLogger.Info("disconnecting connection; exceeded processing deadline")
				} else {
					connLogger.Debug("disconnecting connection; processing failed", zap.Error(err))
				}
				c.Close()
				continue
			}
			c.SetDeadline(time.Time{})
			procConn := pc.(ProcessedConn)

			connLogger.Debug("sending client to server gateway")
			cpn.EventBus.Push(PostConnProcessingEventTopic, PostConnProcessingEvent{
				ProcessedConn: procConn,
			})

			cpn.Out <- procConn
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

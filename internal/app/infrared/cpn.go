package infrared

import (
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

func (cpn CPN) ListenAndServe(quit <-chan bool) {
	for {
		select {
		case c, ok := <-cpn.In:
			if !ok {
				return
			}

			connLogger := cpn.Logger.With(logConn(c)...)
			connLogger.Debug("starting to process connection")
			cpn.EventBus.Push(PreConnProcessingEvent{
				Conn: c,
			}, PreConnProcessingEventTopic)

			//c.SetDeadline(time.Now().Add(cpn.ClientTimeout()))
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
			cpn.EventBus.Push(PostConnProcessingEvent{
				ProcessedConn: procConn,
			}, PostConnProcessingEventTopic)

			cpn.Out <- procConn
		case <-quit:
			return
		}
	}
}

type CPNPool struct {
	CPN CPN

	mu  sync.Mutex
	cfs []chan bool
}

func (p *CPNPool) SetSize(n int) {
	if n < 0 {
		n = 0
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	l := len(p.cfs)
	if l > n {
		p.remove(l - n)
	} else if l < n {
		p.add(n - l)
	}
}

func (p *CPNPool) add(n int) {
	cfs := make([]chan bool, n)
	for i := 0; i < n; i++ {
		quit := make(chan bool)
		go p.CPN.ListenAndServe(quit)
		cfs[i] = quit
	}

	if p.cfs == nil {
		p.cfs = cfs
	} else {
		p.cfs = append(p.cfs, cfs...)
	}
}

func (p *CPNPool) remove(n int) {
	l := len(p.cfs)
	if l == 0 {
		return
	}

	size := l - n
	for i := l - 1; i >= size; i-- {
		p.cfs[i] <- true
		close(p.cfs[i])
	}

	p.cfs = p.cfs[:size]
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
	p.SetSize(0)
}

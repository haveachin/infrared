package infrared

import "github.com/go-logr/logr"

type ConnPool struct {
	Log logr.Logger

	pool []ProcessedConn
}

func (cp *ConnPool) Start(poolChan <-chan ProcessedConn) {
	cp.pool = []ProcessedConn{}

	for {
		c, ok := <-poolChan
		if !ok {
			break
		}

		cp.pool = append(cp.pool, c)
		go c.StartPipe()
	}

	for _, c := range cp.pool {
		c.Close()
	}
}

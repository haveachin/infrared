package main

import (
	"github.com/haveachin/infrared/mc"
	"github.com/haveachin/infrared/mc/protocol"
	"io"
	"os"
	"sync"
)

type gateway struct {
	listener map[string]mc.Listener
	proxies  map[string]*proxy
	wg       *sync.WaitGroup
	logger   io.Writer

	done chan bool
}

func newGateway() gateway {
	return gateway{
		listener: map[string]mc.Listener{},
		proxies:  map[string]*proxy{},
		wg:       &sync.WaitGroup{},
		logger:   os.Stdout,
		done:     make(chan bool, 1),
	}
}

func (gateway *gateway) addProxy(proxy *proxy) {
	if _, ok := gateway.listener[proxy.ListenTo]; !ok {
		go func() {
			_ = gateway.listenAndServe(proxy.ListenTo)
		}()
	}

	gateway.proxies[proxy.uid()] = proxy
}

func (gateway *gateway) listenAndServe(addr string) error {
	l, err := mc.Listen(addr)
	if err != nil {
		return err
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-gateway.done:
				return nil
			default:
				continue
			}
		}

		go gateway.serve(conn, addr)
	}
}

func (gateway *gateway) serve(conn mc.Conn, addr string) {
	packet, err := conn.PeekPacket()
	if err != nil {
		return
	}

	handshake, err := protocol.ParseSLPHandshake(packet)
	if err != nil {
		return
	}

	domain := handshake.ParseServerAddress()
	proxyUID := domain + addr
	proxy, ok := gateway.proxies[proxyUID]
	if !ok {
		return
	}

	_ = proxy.handleConn(conn)
}

// Close closes all gates
func (gateway *gateway) close() error {
	for _, l := range gateway.listener {
		if err := l.Close(); err != nil {
			return err
		}
	}
	return nil
}

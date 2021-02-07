package infrared

import (
	"errors"
	"github.com/specspace/plasma"
	"github.com/specspace/plasma/protocol/packets/handshaking"
	"log"
	"sync"
)

type Gateway struct {
	listeners sync.Map
	proxies   sync.Map
	closed    chan bool
	wg        sync.WaitGroup
}

func (gateway *Gateway) ListenAndServe(proxies []*Proxy) error {
	if len(proxies) <= 0 {
		return errors.New("no proxies in gateway")
	}

	gateway.closed = make(chan bool, len(proxies))

	for _, proxy := range proxies {
		if err := gateway.RegisterProxy(proxy); err != nil {
			gateway.Close()
			return err
		}
	}

	log.Println("All proxies are online")
	gateway.wg.Wait()
	return nil
}

// Close closes all listeners
func (gateway *Gateway) Close() {
	gateway.listeners.Range(func(k, v interface{}) bool {
		gateway.closed <- true
		v.(plasma.Listener).Close()
		return false
	})
}

func (gateway *Gateway) CloseProxy(proxyUID string) {
	log.Println("Closing proxy with UID", proxyUID)
	v, ok := gateway.proxies.LoadAndDelete(proxyUID)
	if !ok {
		return
	}
	proxy := v.(*Proxy)

	closeListener := true
	gateway.proxies.Range(func(k, v interface{}) bool {
		otherProxy := v.(*Proxy)
		if proxy.ListenTo() == otherProxy.ListenTo() {
			closeListener = true
			return false
		}
		return true
	})

	if !closeListener {
		return
	}

	v, ok = gateway.listeners.Load(proxy.ListenTo())
	if ok {
		return
	}
	v.(plasma.Listener).Close()
}

func (gateway *Gateway) RegisterProxy(proxy *Proxy) error {
	// Register new Proxy
	log.Println("Registering proxy with UID", proxy.UID())
	gateway.proxies.Store(proxy.UID(), proxy)
	proxy.Config.OnConfigRemove(func() {
		gateway.CloseProxy(proxy.UID())
	})

	// Check if a gate is already listening to the Proxy address
	if _, ok := gateway.listeners.Load(proxy.ListenTo()); ok {
		return nil
	}

	gateway.wg.Add(1)
	go gateway.listenAndServe(proxy.ListenTo())
	return nil
}

func (gateway *Gateway) listenAndServe(addr string) error {
	defer gateway.wg.Done()

	log.Println("Creating listener on", addr)
	listener, err := plasma.Listen(addr)
	if err != nil {
		return err
	}
	gateway.listeners.Store(addr, listener)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if err.Error() == "use of closed network connection" {
				log.Println("Closing listener on", addr)
				gateway.listeners.Delete(addr)
				// TODO: Event listener closed
				return nil
			}

			// TODO: Event connection failed
			continue
		}

		go func() {
			log.Printf("[>] Incoming %s on listener %s", conn.RemoteAddr(), addr)
			if err := gateway.serve(conn, addr); err != nil {
				log.Printf("[x] %s closed connection with %s; error: %s", conn.RemoteAddr(), addr, err)
				return
			}
			log.Printf("[x] %s closed connection with %s", conn.RemoteAddr(), addr)
			conn.Close()
		}()
	}
}

func (gateway *Gateway) serve(conn plasma.Conn, addr string) error {
	pk, err := conn.PeekPacket()
	if err != nil {
		// TODO: Debug invalid packet format; not a minecraft client?
		return err
	}

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		// TODO: Debug invalid packet send from client
		return err
	}

	proxyUID := proxyUID(hs.ParseServerAddress(), addr)

	log.Printf("[i] %s requests proxy with UID %s", conn.RemoteAddr(), proxyUID)
	v, ok := gateway.proxies.Load(proxyUID)
	if !ok {
		// Client send an invalid address/port; we don't have a v for that address
		// TODO: Log error and show message to client if possible
		return errors.New("no proxy with uid " + proxyUID)
	}
	proxy := v.(*Proxy)

	return proxy.handleConn(conn)
}

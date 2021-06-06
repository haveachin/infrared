package infrared

import (
	"errors"
	"log"
	"sync"

	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/webhook"
	"github.com/pires/go-proxyproto"
)

var (
	ErrCantPeekPK      = errors.New("can't peek packet")
	ErrCantUnMarshalPK = errors.New("can't unmarshal packet")
	ErrUnknownProxy    = errors.New("not a proxy we know")
	ErrUnknownRequeset = errors.New("request isnt status or login, dont know what to do") // Need a better message for this its to specific and code related to this will likely change which will make this message invalid
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
	return nil
}

func (gateway *Gateway) KeepProcessActive() {
	gateway.wg.Wait()
}

// Close closes all listeners
func (gateway *Gateway) Close() {
	gateway.listeners.Range(func(k, v interface{}) bool {
		gateway.closed <- true
		_ = v.(Listener).Close()
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
			closeListener = false
			return false
		}
		return true
	})

	if !closeListener {
		return
	}

	v, ok = gateway.listeners.Load(proxy.ListenTo())
	if !ok {
		return
	}
	v.(Listener).Close()
}

func (gateway *Gateway) RegisterProxy(proxy *Proxy) error {
	// Register new Proxy
	proxyUID := proxy.UID()
	log.Println("Registering proxy with UID", proxyUID)
	gateway.proxies.Store(proxyUID, proxy)

	proxy.Config.removeCallback = func() {
		gateway.CloseProxy(proxyUID)
	}

	proxy.Config.changeCallback = func() {
		if proxyUID == proxy.UID() {
			return
		}
		gateway.CloseProxy(proxyUID)
		if err := gateway.RegisterProxy(proxy); err != nil {
			log.Println(err)
		}
	}

	// Check if a gate is already listening to the Proxy address
	addr := proxy.ListenTo()
	if _, ok := gateway.listeners.Load(addr); ok {
		return nil
	}

	log.Println("Creating listener on", addr)
	listener, err := Listen(addr)
	if err != nil {
		return err
	}
	gateway.listeners.Store(addr, listener)

	gateway.wg.Add(1)
	go func() {
		if err := gateway.listenAndServe(listener, addr); err != nil {
			log.Printf("Failed to listen on %s; error: %s", proxy.ListenTo(), err)
		}
	}()
	return nil
}

func (gateway *Gateway) listenAndServe(listener Listener, addr string) error {
	defer gateway.wg.Done()

	for {
		conn, err := listener.Accept()
		if err != nil {
			// TODO: Refactor this; it feels hacky
			if err.Error() == "use of closed network connection" {
				log.Println("Closing listener on", addr)
				gateway.listeners.Delete(addr)
				return nil
			}

			continue
		}

		go func() {
			log.Printf("[>] Incoming %s on listener %s", conn.RemoteAddr(), addr)
			defer conn.Close()
			if err := gateway.serve(conn, addr); err != nil {
				log.Printf("[x] %s closed connection with %s; error: %s", conn.RemoteAddr(), addr, err)
				return
			}
			log.Printf("[x] %s closed connection with %s", conn.RemoteAddr(), addr)
		}()
	}
}

func (gateway *Gateway) serve(conn Conn, addr string) error {
	pk, err := conn.PeekPacket()
	if err != nil {
		return ErrCantPeekPK
	}

	connRemoteAddr := conn.RemoteAddr()
	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		header, err := proxyproto.Read(conn.Reader())
		if err != nil {
			return err
		}
		connRemoteAddr = header.SourceAddr
		pk, err := conn.PeekPacket()
		if err != nil {
			return ErrCantPeekPK
		}
		hs, err = handshaking.UnmarshalServerBoundHandshake(pk)
		if err != nil {
			return ErrCantUnMarshalPK
		}
	}

	proxyUID := proxyUID(hs.ParseServerAddress(), addr)

	log.Printf("[i] %s requests proxy with UID %s", connRemoteAddr, proxyUID)
	v, ok := gateway.proxies.Load(proxyUID)
	if !ok {
		// Client send an invalid address/port; we don't have a v for that address
		return ErrUnknownProxy
	}
	proxy := v.(*Proxy)

	if err := proxy.handleConn(conn, connRemoteAddr); err != nil {
		proxy.CallbackLogger().SendEvent(webhook.ErrorEvent{
			Error:    err.Error(),
			ProxyUID: proxyUID,
		})
		return err
	}
	return nil
}

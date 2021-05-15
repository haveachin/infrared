package infrared

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/haveachin/infrared/callback"
	"github.com/haveachin/infrared/protocol/handshaking"
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
		proxy.CallbackLogger().LogEvent(callback.ErrorEvent{
			Error:    err.Error(),
			ProxyUID: proxyUID,
		})
		return err
	}
	return nil
}

// Make them the same value as the ServerBoundHS states
const (
	_ int8 = iota
	StatusRequest
	LoginRequest
	Unknown
)

type Connection interface {
	remoteAddr() net.Addr
	proxyUID() (string, error)
	isRequestType(requestType int8) bool
	proxyToServer(server MCServer)
	establishProxy(proxy *Proxy) error


	Conn() Conn //Temp method ment for implementation MUST be removed later
}

type PlayerConnection struct {
	mcName      string
	addr        net.Addr
	targetUID   string
	requestType int8
	conn        Conn
	s 		MCServer
}

func (pConn *PlayerConnection) proxyToServer(server MCServer) {
	pConn.s = server
}

func (pConn *PlayerConnection) Conn() Conn {
	return pConn.conn
}

func (pConn *PlayerConnection) server() MCServer {
	return pConn.s
}

func (pConn *PlayerConnection) isRequestType(requestType int8) bool {
	return requestType == pConn.requestType
}

func (pConn *PlayerConnection) remoteAddr() net.Addr {
	if pConn.addr == nil {
		pConn.addr = pConn.conn.RemoteAddr()
	}
	return pConn.addr
}

//Method does too much...?
func (pConn *PlayerConnection) proxyUID() (string, error) {
	if pConn.targetUID != "" {
		return pConn.targetUID, nil
	}
	pk, err := pConn.conn.PeekPacket()
	if err != nil {
		return "", ErrCantPeekPK
	}

	connRemoteAddr := pConn.conn.RemoteAddr()
	pConn.addr = connRemoteAddr
	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		header, err := proxyproto.Read(pConn.conn.Reader())
		if err != nil {
			return "", err
		}
		pConn.addr = header.SourceAddr

		pk, err := pConn.conn.PeekPacket()
		if err != nil {
			return "", ErrCantPeekPK
		}
		hs, err = handshaking.UnmarshalServerBoundHandshake(pk)
		if err != nil {
			return "", ErrCantUnMarshalPK
		}
	}

	switch {
	case hs.IsStatusRequest():
		pConn.requestType = StatusRequest
	case hs.IsLoginRequest():
		pConn.requestType = LoginRequest
	default:
		pConn.requestType = Unknown
	}

	pConn.targetUID = proxyUID(hs.ParseServerAddress(), pConn.addr.String())
	log.Printf("[i] %s requests proxy with UID %s", pConn.addr, pConn.targetUID)

	return pConn.targetUID, nil
}

func (pConn *PlayerConnection) sendHSPacket(proxy *Proxy) error {
	pk, err := pConn.conn.ReadPacket()
	if err != nil {
		return ErrCantReadPK
	}

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		return ErrCantUnMarshalPK
	}

	if proxy.RealIP() {
		hs.UpgradeToRealIP(pConn.remoteAddr(), time.Now())
		pk = hs.Marshal()
	}

	rconn, _ := pConn.server().Connection()
	if err := rconn.WritePacket(pk); err != nil {
		return ErrCantWriteToServer
	}
	return nil
}

func (pConn *PlayerConnection) establishProxy(proxy *Proxy) error {

	server := proxy.ServerFactory(proxy)
	pConn.proxyToServer(server)

	rconn, err := proxy.server.Connection()
	if err != nil {
		return ErrCantConnectWithServer
	}
	defer rconn.Close()

	if proxy.ProxyProtocol() {
		header := &proxyproto.Header{
			Version:           2,
			Command:           proxyproto.PROXY,
			TransportProtocol: proxyproto.TCPv4,
			SourceAddr:        pConn.remoteAddr(),
			DestinationAddr:   rconn.RemoteAddr(),
		}

		if _, err = header.WriteTo(rconn); err != nil {
			return ErrCantWriteToServer
		}
	}

	pConn.sendHSPacket(proxy)

	proxyUID := proxy.UID()
	proxyTo := proxy.ProxyTo()
	connRemoteAddr := pConn.remoteAddr()

	// rconn, _ := pConn.server().Connection()


	var username string
	proxy.cancelProcessTimeout()
	username, err = sniffUsername(pConn, rconn)
	if err != nil {
		return err
	}
	proxy.addPlayer(pConn.Conn(), username)
	proxy.logEvent(callback.PlayerJoinEvent{
		Username:      username,
		RemoteAddress: connRemoteAddr.String(),
		TargetAddress: proxyTo,
		ProxyUID:      proxyUID,
	})

	go pipe(rconn, pConn.Conn())
	pipe(pConn.Conn(), rconn)

	proxy.logEvent(callback.PlayerLeaveEvent{
		Username:      username,
		RemoteAddress: pConn.remoteAddr().String(),
		TargetAddress: proxyTo,
		ProxyUID:      proxyUID,
	})

	// remainingPlayers := proxy.removePlayer(conn)
	// if remainingPlayers <= 0 {
	// 	proxy.timeoutProcess()
	// }
	return nil

}

func (gateway *Gateway) serve2(conn Connection, addr string) error {
	proxyUID, err := conn.proxyUID()
	if err != nil {
		return err
	}

	v, ok := gateway.proxies.Load(proxyUID)
	if !ok {
		// Client send an invalid address/port; we don't have a v for that address
		return ErrUnknownProxy
	}
	proxy := v.(*Proxy)

	if err := handleConn(conn, proxy); err != nil {
		proxy.CallbackLogger().LogEvent(callback.ErrorEvent{
			Error:    err.Error(),
			ProxyUID: proxyUID,
		})
		return err
	}
	return nil
}

func handleConn(conn Connection, proxy *Proxy) error {
	proxyToServer := proxy.ServerFactory(proxy)
	conn.proxyToServer(proxyToServer)
	if !proxyToServer.CanConnect() {
		if conn.isRequestType(StatusRequest) {
			return proxy.handleStatusRequest(conn.Conn(), false)
		}
		if err := proxy.startProcessIfNotRunning(); err != nil {
			return err
		}
		proxy.timeoutProcess()
		return proxy.handleLoginRequest(conn.Conn())
	}

	if conn.isRequestType(StatusRequest) && proxy.IsOnlineStatusConfigured() {
		return proxy.handleStatusRequest(conn.Conn(), true)
	}

	if !conn.isRequestType(LoginRequest) {
		return nil
	}
	
	return conn.establishProxy(proxy)
}

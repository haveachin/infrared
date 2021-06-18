package infrared

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/haveachin/infrared/callback"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/status"
	"github.com/pires/go-proxyproto"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	proxiesActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "infrared_proxies",
		Help: "The total number of proxies running",
	})

	responsePk protocol.Packet
)

func init() {
	responseJSON := status.ResponseJSON{
		Version: status.VersionJSON{
			Name:     "",
			Protocol: 0,
		},
		Players: status.PlayersJSON{
			Max:    0,
			Online: 0,
		},
		Description: status.DescriptionJSON{
			Text: "Powered by ...Hosting",
		},
	}
	bb, _ := json.Marshal(responseJSON)

	responsePk = status.ClientBoundResponse{
		JSONResponse: protocol.String(bb),
	}.Marshal()
}

type Gateway struct {
	listeners            sync.Map
	proxies              sync.Map
	closed               chan bool
	wg                   sync.WaitGroup
	receiveProxyProtocol bool
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

func (gateway *Gateway) EnablePrometheus(bind string) error {
	gateway.wg.Add(1)

	go func() {
		defer gateway.wg.Done()

		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(bind, nil)
	}()

	log.Println("Enabling Prometheus metrics endpoint on", bind)
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
	proxiesActive.Dec()
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
	uids := proxy.UIDs()
	for _, uid := range uids {
		log.Println("Registering proxy with UID", uid)
		gateway.proxies.Store(uid, proxy)
	}
	proxyUID := proxy.UID()
	proxiesActive.Inc()

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

	playersConnected.WithLabelValues(proxy.DomainName())

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
	connRemoteAddr := conn.RemoteAddr()
	if gateway.receiveProxyProtocol {
		header, err := proxyproto.Read(conn.Reader())
		if err != nil {
			return err
		}
		connRemoteAddr = header.SourceAddr
	}

	pk, err := conn.PeekPacket()
	if err != nil {
		return err
	}

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		return err
	}

	proxyUID := proxyUID(hs.ParseServerAddress(), addr)

	log.Printf("[i] %s requests proxy with UID %s", connRemoteAddr, proxyUID)
	v, ok := gateway.proxies.Load(proxyUID)
	if !ok {
		if hs.IsStatusRequest() {
			conn.ReadPacket()
			conn.WritePacket(responsePk)
			pingPk, _ := conn.ReadPacket()
			conn.WritePacket(pingPk)
		}

		// Client send an invalid address/port; we don't have a v for that address
		return errors.New("no proxy with uid " + proxyUID)
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

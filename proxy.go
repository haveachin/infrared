package infrared

import (
	"fmt"
	"github.com/haveachin/infrared/process"
	"github.com/specspace/plasma"
	"github.com/specspace/plasma/protocol"
	"github.com/specspace/plasma/protocol/packets/handshaking"
	"github.com/specspace/plasma/protocol/packets/login"
	"github.com/specspace/plasma/protocol/packets/status"
	"log"
	"strings"
	"sync"
	"time"
)

func proxyUID(domain, addr string) string {
	return fmt.Sprintf("%s@%s", strings.ToLower(domain), addr)
}

type Proxy struct {
	Players sync.Map
	Config  *ProxyConfig

	process              process.Process
	statusResponsePacket *protocol.Packet
}

func (proxy *Proxy) DomainName() string {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.DomainName
}

func (proxy *Proxy) ListenTo() string {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.ListenTo
}

func (proxy *Proxy) ProxyTo() string {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.ProxyTo
}

func (proxy *Proxy) DisconnectMessage() string {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.DisconnectMessage
}

func (proxy *Proxy) OnlineStatusPacket() (protocol.Packet, error) {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	if proxy.Config.onlineStatusPacket != nil {
		return *proxy.Config.onlineStatusPacket, nil
	}

	bb, err := proxy.Config.OnlineStatus.StatusResponse().JSON()
	if err != nil {
		return protocol.Packet{}, err
	}

	pk := status.ClientBoundResponse{
		JSONResponse: protocol.String(bb),
	}.Marshal()
	proxy.Config.onlineStatusPacket = &pk
	return pk, nil
}

func (proxy *Proxy) OfflineStatusPacket() (protocol.Packet, error) {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	if proxy.Config.offlineStatusPacket != nil {
		return *proxy.Config.offlineStatusPacket, nil
	}

	bb, err := proxy.Config.OfflineStatus.StatusResponse().JSON()
	if err != nil {
		return protocol.Packet{}, err
	}

	pk := status.ClientBoundResponse{
		JSONResponse: protocol.String(bb),
	}.Marshal()
	proxy.Config.offlineStatusPacket = &pk
	return pk, nil
}

func (proxy *Proxy) Timeout() time.Duration {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return time.Millisecond * time.Duration(proxy.Config.Timeout)
}

func (proxy *Proxy) UID() string {
	return proxyUID(proxy.DomainName(), proxy.ListenTo())
}

func (proxy *Proxy) handleConn(conn plasma.Conn) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		// TODO: Debug invalid packet format; not a minecraft client?
		return err
	}

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		// TODO: Debug invalid packet send from client
		return err
	}

	rconn, err := plasma.DialTimeout(proxy.ProxyTo(), proxy.Timeout())
	if err != nil {
		log.Printf("[i] %s did not respond to ping; is the target offline?", proxy.ProxyTo())
		if err := proxy.startProcessIfNotRunning(); err != nil {
			return err
		}
		if hs.IsLoginRequest() {
			return proxy.handleLoginRequest(conn)
		}
		return proxy.handleStatusRequest(conn)
	}
	defer rconn.Close()

	if err := rconn.WritePacket(pk); err != nil {
		return err
	}

	if hs.IsLoginRequest() {
		if err := proxy.sniffUsername(conn, rconn); err != nil {
			return err
		}
	}

	go pipe(rconn, conn)
	pipe(conn, rconn)
	return nil
}

func pipe(src, dst plasma.Conn) {
	buffer := make([]byte, 0xffff)

	for {
		n, err := src.Read(buffer)
		if err != nil {
			return
		}

		data := buffer[:n]

		_, err = dst.Write(data)
		if err != nil {
			return
		}
	}
}

func (proxy *Proxy) startProcessIfNotRunning() error {
	if proxy.process == nil {
		return nil
	}

	running, err := proxy.process.IsRunning()
	if err != nil {
		return err
	}

	if !running {
		return proxy.process.Start()
	}

	return nil
}

func (proxy *Proxy) sniffUsername(conn, rconn plasma.Conn) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return err
	}
	rconn.WritePacket(pk)

	ls, err := login.UnmarshalServerBoundLoginStart(pk)
	if err != nil {
		return err
	}
	proxy.Players.Store(conn, string(ls.Name))
	log.Printf("[i] %s with username %s connects through %s", conn.RemoteAddr(), ls.Name, proxy.UID())
	return nil
}

func (proxy *Proxy) handleLoginRequest(conn plasma.Conn) error {
	packet, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	loginStart, err := login.UnmarshalServerBoundLoginStart(packet)
	if err != nil {
		return err
	}

	message := proxy.DisconnectMessage()
	templates := map[string]string{
		"username":      string(loginStart.Name),
		"now":           time.Now().Format(time.RFC822),
		"remoteAddress": conn.LocalAddr().String(),
		"localAddress":  conn.LocalAddr().String(),
		"domain":        proxy.DomainName(),
		"proxyTo":       proxy.ProxyTo(),
		"listenTo":      proxy.ListenTo(),
	}

	for key, value := range templates {
		message = strings.Replace(message, fmt.Sprintf("{{%s}}", key), value, -1)
	}

	return conn.WritePacket(login.ClientBoundDisconnect{
		Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", message)),
	}.Marshal())
}

func (proxy *Proxy) handleStatusRequest(conn plasma.Conn) error {
	// Read the request packet and send status response back
	_, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	responsePk, err := proxy.OfflineStatusPacket()
	if err != nil {
		return err
	}

	if err := conn.WritePacket(responsePk); err != nil {
		return err
	}

	pingPk, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	return conn.WritePacket(pingPk)
}

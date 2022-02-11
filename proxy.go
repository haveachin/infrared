package infrared

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/haveachin/infrared/callback"
	"github.com/haveachin/infrared/process"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/login"
	"github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	playersConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infrared_connected",
		Help: "The total number of connected players",
	}, []string{"host"})
)

func proxyUID(domain, addr string) string {
	return fmt.Sprintf("%s@%s", strings.ToLower(domain), addr)
}

type Proxy struct {
	Config *ProxyConfig

	cancelTimeoutFunc func()
	players           map[Conn]string
	mu                sync.Mutex
}

func (proxy *Proxy) Process() process.Process {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	if proxy.Config.process != nil {
		return proxy.Config.process
	}

	if proxy.Config.Docker.IsPortainer() {
		portainer, err := process.NewPortainer(
			proxy.Config.Docker.ContainerName,
			proxy.Config.Docker.Portainer.Address,
			proxy.Config.Docker.Portainer.EndpointID,
			proxy.Config.Docker.Portainer.Username,
			proxy.Config.Docker.Portainer.Password,
		)
		if err != nil {
			log.Println("Failed to create a Portainer process; error:", err)
			return nil
		}
		proxy.Config.process = portainer
		return portainer
	}

	if proxy.Config.Docker.IsDocker() {
		docker, err := process.NewDocker(proxy.Config.Docker.ContainerName)
		if err != nil {
			log.Println("Failed to create a Docker process; error:", err)
			return nil
		}
		proxy.Config.process = docker
		return docker
	}

	return nil
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

func (proxy *Proxy) Dialer() (*Dialer, error) {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.Dialer()
}

func (proxy *Proxy) DisconnectMessage() string {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.DisconnectMessage
}

func (proxy *Proxy) IsOnlineStatusConfigured() bool {
	proxy.Config.Lock()
	defer proxy.Config.Unlock()
	return proxy.Config.OnlineStatus.ProtocolNumber != 0
}

func (proxy *Proxy) OnlineStatusPacket() (protocol.Packet, error) {
	proxy.Config.Lock()
	defer proxy.Config.Unlock()
	return proxy.Config.OnlineStatus.StatusResponsePacket()
}

func (proxy *Proxy) OfflineStatusPacket() (protocol.Packet, error) {
	proxy.Config.Lock()
	defer proxy.Config.Unlock()
	return proxy.Config.OfflineStatus.StatusResponsePacket()
}

func (proxy *Proxy) Timeout() time.Duration {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return time.Millisecond * time.Duration(proxy.Config.Timeout)
}

func (proxy *Proxy) DockerTimeout() time.Duration {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return time.Millisecond * time.Duration(proxy.Config.Docker.Timeout)
}

func (proxy *Proxy) SpoofForcedHost() string {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.SpoofForcedHost
}

func (proxy *Proxy) ProxyProtocol() bool {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.ProxyProtocol
}

func (proxy *Proxy) RealIP() bool {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return proxy.Config.RealIP
}

func (proxy *Proxy) CallbackLogger() callback.Logger {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return callback.Logger{
		URL:    proxy.Config.CallbackServer.URL,
		Events: proxy.Config.CallbackServer.Events,
	}
}

func (proxy *Proxy) UID() string {
	return proxyUID(proxy.DomainName(), proxy.ListenTo())
}

func (proxy *Proxy) addPlayer(conn Conn, username string) {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if proxy.players == nil {
		proxy.players = map[Conn]string{}
	}
	proxy.players[conn] = username
}

func (proxy *Proxy) removePlayer(conn Conn) int {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if proxy.players == nil {
		proxy.players = map[Conn]string{}
		return 0
	}
	delete(proxy.players, conn)
	return len(proxy.players)
}

func (proxy *Proxy) logEvent(event callback.Event) {
	if _, err := proxy.CallbackLogger().LogEvent(event); err != nil {
		log.Println("[w] Failed callback logging; error:", err)
	}
}

func (proxy *Proxy) handleConn(conn Conn, connRemoteAddr net.Addr) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		return err
	}

	proxyDomain := proxy.DomainName()
	proxyTo := proxy.ProxyTo()
	proxyUID := proxy.UID()

	dialer, err := proxy.Dialer()
	if err != nil {
		return err
	}

	rconn, err := dialer.Dial(proxyTo)
	if err != nil {
		log.Printf("[i] %s did not respond to ping; is the target offline?", proxyTo)
		if hs.IsStatusRequest() {
			return proxy.handleStatusRequest(conn, false)
		}
		if err := proxy.startProcessIfNotRunning(); err != nil {
			return err
		}
		proxy.timeoutProcess()
		return proxy.handleLoginRequest(conn)
	}
	defer rconn.Close()

	if hs.IsStatusRequest() && proxy.IsOnlineStatusConfigured() {
		return proxy.handleStatusRequest(conn, true)
	}

	spoofForcedHost := proxy.SpoofForcedHost()
	if spoofForcedHost != "" {
		hs.ServerAddress = protocol.String(spoofForcedHost)
		pk = hs.Marshal()
	}

	if proxy.ProxyProtocol() {
		header := &proxyproto.Header{
			Version:           2,
			Command:           proxyproto.PROXY,
			TransportProtocol: proxyproto.TCPv4,
			SourceAddr:        connRemoteAddr,
			DestinationAddr:   rconn.RemoteAddr(),
		}

		if _, err = header.WriteTo(rconn); err != nil {
			return err
		}
	}

	if proxy.RealIP() {
		hs.UpgradeToRealIP(connRemoteAddr, time.Now())
		pk = hs.Marshal()
	}

	if err := rconn.WritePacket(pk); err != nil {
		return err
	}

	var username string
	connected := false
	if hs.IsLoginRequest() {
		proxy.cancelProcessTimeout()
		username, err = proxy.sniffUsername(conn, rconn, connRemoteAddr)
		if err != nil {
			return err
		}
		proxy.addPlayer(conn, username)
		proxy.logEvent(callback.PlayerJoinEvent{
			Username:      username,
			RemoteAddress: connRemoteAddr.String(),
			TargetAddress: proxyTo,
			ProxyUID:      proxyUID,
		})
		playersConnected.With(prometheus.Labels{"host": proxyDomain}).Inc()
		connected = true
	}

	go pipe(rconn, conn)
	pipe(conn, rconn)

	if connected {
		proxy.logEvent(callback.PlayerLeaveEvent{
			Username:      username,
			RemoteAddress: connRemoteAddr.String(),
			TargetAddress: proxyTo,
			ProxyUID:      proxyUID,
		})
		playersConnected.With(prometheus.Labels{"host": proxyDomain}).Dec()
	}

	remainingPlayers := proxy.removePlayer(conn)
	if remainingPlayers <= 0 {
		proxy.timeoutProcess()
	}
	return nil
}

func pipe(src, dst Conn) {
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
	if proxy.Process() == nil {
		return nil
	}

	running, err := proxy.Process().IsRunning()
	if err != nil {
		return err
	}

	if running {
		return nil
	}

	log.Println("[i] Starting container for", proxy.UID())
	proxy.logEvent(callback.ContainerStartEvent{ProxyUID: proxy.UID()})
	return proxy.Process().Start()
}

func (proxy *Proxy) timeoutProcess() {
	if proxy.Process() == nil {
		return
	}

	if proxy.DockerTimeout() <= 0 {
		return
	}

	proxy.cancelProcessTimeout()

	log.Printf("[i] Starting container timeout %s on %s", proxy.DockerTimeout(), proxy.UID())
	timer := time.AfterFunc(proxy.DockerTimeout(), func() {
		log.Println("[i] Stopping container on", proxy.UID())
		proxy.logEvent(callback.ContainerStopEvent{ProxyUID: proxy.UID()})
		if err := proxy.Process().Stop(); err != nil {
			log.Printf("[w] Failed to stop the container for %s; error: %s", proxy.UID(), err)
		}
	})

	proxy.cancelTimeoutFunc = func() {
		if timer.Stop() {
			log.Println("[i] Timout stopped for", proxy.UID())
		}
	}
}

func (proxy *Proxy) cancelProcessTimeout() {
	if proxy.cancelTimeoutFunc == nil {
		return
	}

	proxy.cancelTimeoutFunc()
	proxy.cancelTimeoutFunc = nil
}

func (proxy *Proxy) sniffUsername(conn, rconn Conn, connRemoteAddr net.Addr) (string, error) {
	pk, err := conn.ReadPacket()
	if err != nil {
		return "", err
	}
	rconn.WritePacket(pk)

	ls, err := login.UnmarshalServerBoundLoginStart(pk)
	if err != nil {
		return "", err
	}
	log.Printf("[i] %s with username %s connects through %s", connRemoteAddr, ls.Name, proxy.UID())
	return string(ls.Name), nil
}

func (proxy *Proxy) handleLoginRequest(conn Conn) error {
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

func (proxy *Proxy) handleStatusRequest(conn Conn, online bool) error {
	// Read the request packet and send status response back
	_, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	var responsePk protocol.Packet
	if online {
		responsePk, err = proxy.OnlineStatusPacket()
		if err != nil {
			return err
		}
	} else {
		responsePk, err = proxy.OfflineStatusPacket()
		if err != nil {
			return err
		}
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

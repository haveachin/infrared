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
	Config *ProxyConfig

	cancelTimeoutFunc func()
	players           map[plasma.Conn]string
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

func (proxy *Proxy) DockerTimeout() time.Duration {
	proxy.Config.RLock()
	defer proxy.Config.RUnlock()
	return time.Millisecond * time.Duration(proxy.Config.Docker.Timeout)
}

func (proxy *Proxy) UID() string {
	return proxyUID(proxy.DomainName(), proxy.ListenTo())
}

func (proxy *Proxy) addPlayer(conn plasma.Conn, username string) {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if proxy.players == nil {
		proxy.players = map[plasma.Conn]string{}
	}
	proxy.players[conn] = username
}

func (proxy *Proxy) removePlayer(conn plasma.Conn) int {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if proxy.players == nil {
		proxy.players = map[plasma.Conn]string{}
		return 0
	}
	delete(proxy.players, conn)
	return len(proxy.players)
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

	if hs.IsStatusRequest() {
		return proxy.handleStatusRequest(conn, true)
	}

	if err := rconn.WritePacket(pk); err != nil {
		return err
	}

	username, err := proxy.sniffUsername(conn, rconn)
	if err != nil {
		return err
	}
	proxy.addPlayer(conn, username)

	proxy.cancelProcessTimeout()

	go pipe(rconn, conn)
	pipe(conn, rconn)

	proxy.mu.Lock()
	proxy.mu.Unlock()

	if proxy.removePlayer(conn) <= 0 {
		proxy.timeoutProcess()
	}
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
	return proxy.Process().Start()
}

func (proxy *Proxy) timeoutProcess() {
	if proxy.Process() == nil {
		return
	}

	if proxy.DockerTimeout() <= 0 {
		return
	}

	log.Printf("[i] Starting container timeout %s on %s", proxy.DockerTimeout(), proxy.UID())
	timer := time.AfterFunc(proxy.DockerTimeout(), func() {
		log.Println("[i] Stopping container on", proxy.UID())
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

func (proxy *Proxy) sniffUsername(conn, rconn plasma.Conn) (string, error) {
	pk, err := conn.ReadPacket()
	if err != nil {
		return "", err
	}
	rconn.WritePacket(pk)

	ls, err := login.UnmarshalServerBoundLoginStart(pk)
	if err != nil {
		return "", err
	}
	log.Printf("[i] %s with username %s connects through %s", conn.RemoteAddr(), ls.Name, proxy.UID())
	return string(ls.Name), nil
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

func (proxy *Proxy) handleStatusRequest(conn plasma.Conn, online bool) error {
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

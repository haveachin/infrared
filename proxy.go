package infrared

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Tnze/go-mc/net"
	"github.com/haveachin/infrared/config"
	"github.com/haveachin/infrared/process"
	"github.com/haveachin/infrared/wrapper"
	proxy "github.com/jpillora/go-tcp-proxy"

	"github.com/Tnze/go-mc/net/packet"
)

// Proxy is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Proxy struct {
	proxy             proxy.Proxy
	DomainName        string
	ListenTo          string
	ProxyTo           string
	Timeout           time.Duration
	Players           map[*net.Conn]string
	Process           process.Process
	placeholderPacket packet.Packet
	close             chan bool
	hasRunningProcess bool
	isTimingOut       bool
	cancelTimeout     chan bool
}

// NewProxy takes a config an creates a new proxy based on it
func NewProxy(cfg config.Config) (*Proxy, error) {
	placeholderBytes, err := cfg.MarshalPlaceholder()
	if err != nil {
		return nil, fmt.Errorf("could not mashal palceholder for %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
	}

	proc, err := processFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not load process for %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
	}

	_ = proc.Stop()

	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("could not parse timeout for %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
	}

	return &Proxy{
		proxy:      proxy.New(),
		DomainName: cfg.DomainName,
		ListenTo:   cfg.ListenTo,
		ProxyTo:    cfg.ProxyTo,
		Timeout:    timeout,
		Players:    map[*net.Conn]string{},
		Process:    proc,
		placeholderPacket: packet.SLPResponse{
			JSONResponse: packet.String(placeholderBytes),
		}.Marshal(),
		hasRunningProcess: false,
		isTimingOut:       false,
		cancelTimeout:     make(chan bool, 1),
	}, nil
}

// HandleConn takes a minecraft client connection and it's initial handschake packet
// and relays all following packets to the remote connection (ProxyTo)
func (p *Proxy) HandleConn(conn *net.Conn, handshake wrapper.SLPHandshake) error {
	defer conn.Close()

	if !p.hasRunningProcess {
		if handshake.RequestsLogin() {
			log.Printf("Process[%s|%s]: starting", p.DomainName, p.ListenTo)

			if err := p.Process.Start(); err != nil {
				log.Printf("Process[%s|%s]: %s", p.DomainName, p.ListenTo, err)
			} else {
				p.hasRunningProcess = true
				go p.doTimeout()
			}
		}

		p.handleServerListPing(conn, handshake)
		return nil
	}

	rconn, err := net.DialMC(p.ProxyTo)
	if err != nil {
		p.handleServerListPing(conn, handshake)
		return err
	}
	defer rconn.Close()

	var pipe = func(src, dst *net.Conn) {
		defer func() {
			if src == conn {
				return
			}

			//log.Printf("%s[%s] lost connection", p.Players[conn], conn.Addr)
			delete(p.Players, conn)
			go p.doTimeout()
			p.close <- true
		}()

		buffer := make([]byte, 0xffff)

		for {
			n, err := src.Socket.Read(buffer)
			if err != nil {
				return
			}

			data := buffer[:n]

			_, _ = dst.Socket.Write(data)
		}
	}

	if handshake.RequestsStatus() {
		if err := rconn.WritePacket(handshake.Marshal()); err != nil {
			return fmt.Errorf("failed to write handshake packet to [%s]", rconn.Addr)
		}

		pk, err := conn.ReadPacket()
		if err != nil {
			return fmt.Errorf("failed to read request packet from [%s]", conn.Addr)
		}

		if err := rconn.WritePacket(pk); err != nil {
			return fmt.Errorf("failed to write request packet to [%s]", rconn.Addr)
		}
	} else if handshake.RequestsLogin() {
		if err := rconn.WritePacket(handshake.Marshal()); err != nil {
			return fmt.Errorf("failed to write handshake packet to [%s]", rconn.Addr)
		}

		username, err := sniffUsername(conn, rconn)
		if err != nil {
			return fmt.Errorf("failed to sniff username from [%s]", conn.Addr)
		}

		if p.isTimingOut {
			p.cancelTimeout <- true
		}

		p.Players[conn] = username

		go pipe(conn, rconn)
		go pipe(rconn, conn)

		return fmt.Errorf("%s[%s] connected over %s[%s]", username, conn.Addr, p.DomainName, p.ListenTo)
	}

	go pipe(conn, rconn)
	go pipe(rconn, conn)

	<-p.close

	return nil
}

// ApplyConfigChange is a callback function that handles config changes
func (p *Proxy) ApplyConfigChange(cfg config.Config) error {
	placeholderBytes, err := cfg.MarshalPlaceholder()
	if err != nil {
		return fmt.Errorf("%s[%s] could not mashal palceholder; %s", p.DomainName, p.ListenTo, err)
	}

	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return fmt.Errorf("%s[%s] could not parse timeout; %s", p.DomainName, p.ListenTo, err)
	}

	p.DomainName = cfg.DomainName
	p.ListenTo = cfg.ListenTo
	p.ProxyTo = cfg.ProxyTo
	p.Timeout = timeout
	p.placeholderPacket = packet.SLPResponse{
		JSONResponse: packet.String(placeholderBytes),
	}.Marshal()

	return nil
}

func processFromConfig(cfg config.Config) (process.Process, error) {
	if cfg.Command != "" {
		return process.NewCommand(cfg.Command), nil
	}

	if cfg.UsesPortainer() {
		dCfg := cfg.Docker
		pCfg := dCfg.Portainer
		return process.NewPortainer(pCfg.Address, pCfg.EndpointID, dCfg.ContainerID, pCfg.Username, pCfg.Password)
	}

	if cfg.UsesDocker() {
		return process.NewDocker(cfg.Docker.ContainerID)
	}

	return nil, errors.New("no process declared")
}

func (p *Proxy) doTimeout() {
	if p.isTimingOut {
		return
	}

	p.isTimingOut = true

	if len(p.Players) > 0 {
		return
	}

	select {
	case <-p.cancelTimeout:
		p.isTimingOut = false
		return
	case <-time.After(p.Timeout):
		log.Printf("Process[%s|%s]: timed out; stopping process", p.DomainName, p.ListenTo)
		if err := p.Process.Stop(); err != nil {
			log.Printf("Process[%s|%s]: %s", p.DomainName, p.ListenTo, err)
		}
		p.hasRunningProcess = false
		return
	}
}

func (p Proxy) handleServerListPing(conn *net.Conn, handshake packet.SLPHandshake) error {
	if handshake.RequestsLogin() {
		return p.handleLoginState(conn)
	}

	return p.handleStatusState(conn)
}

func (p Proxy) handleStatusState(conn *net.Conn) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	if pk.ID != packet.SLPRequestPacketID {
		return fmt.Errorf("expexted request packet \"%d\"; got this %d", packet.SLPRequestPacketID, pk.ID)
	}

	if err := conn.WritePacket(p.placeholderPacket); err != nil {
		return err
	}

	pk, err = conn.ReadPacket()
	if err != nil {
		return err
	}

	if pk.ID != packet.SLPPingPacketID {
		return fmt.Errorf("expexted ping packet id \"%d\"; got this %d", packet.SLPPingPacketID, pk.ID)
	}

	return conn.WritePacket(pk)
}

func (p Proxy) handleLoginState(conn *net.Conn) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return err
	}

	start, err := packet.ParseLoginStart(pk)
	if err != nil {
		return err
	}

	message := strings.Replace(p.DisconnectMessage, "$username", string(start.Name), -1)
	message = fmt.Sprintf("{\"text\":\"%s\"}", message)

	disconnect := packet.LoginDisconnect{
		Reason: packet.Chat(message),
	}

	return conn.WritePacket(disconnect.Marshal())
}

func sniffUsername(conn, rconn *net.Conn) (string, error) {
	pk, err := conn.ReadPacket()
	if err != nil {
		return "", err
	}

	start, err := packet.ParseLoginStart(pk)
	if err != nil {
		return "", err
	}

	if err := rconn.WritePacket(pk); err != nil {
		return "", err
	}

	return string(start.Name), nil
}

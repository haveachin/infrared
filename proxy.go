package infrared

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/haveachin/infrared/config"
	"github.com/haveachin/infrared/net"
	"github.com/haveachin/infrared/process"

	"github.com/haveachin/infrared/net/packet"
)

// Proxy is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Proxy struct {
	DomainName        string
	ListenTo          string
	ProxyTo           string
	DisconnectMessage string
	Timeout           time.Duration
	Players           map[*net.Conn]string
	Process           process.Process
	placeholderPacket packet.Packet
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
		DomainName:        cfg.DomainName,
		ListenTo:          cfg.ListenTo,
		ProxyTo:           cfg.ProxyTo,
		DisconnectMessage: cfg.DisconnectMessage,
		Timeout:           timeout,
		Players:           map[*net.Conn]string{},
		Process:           proc,
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
func (p *Proxy) HandleConn(conn *net.Conn, handshake packet.SLPHandshake) error {
	if !p.hasRunningProcess {
		if handshake.RequestsLogin() {
			log.Printf("Process[%s:%s]: starting", p.DomainName, p.ListenTo)

			if err := p.Process.Start(); err != nil {
				log.Printf("Process[%s:%s]: %s", p.DomainName, p.ListenTo, err)
			} else {
				p.hasRunningProcess = true
				go p.doTimeout()
			}
		}

		p.simulateServer(conn, handshake)
		return nil
	}

	rconn, err := net.DialMC(p.ProxyTo)
	if err != nil {
		p.simulateServer(conn, handshake)
		return err
	}

	var pipe = func(src, dst *net.Conn) {
		defer func() {
			delete(p.Players, conn)
			_ = conn.Close()
			_ = rconn.Close()
			go p.doTimeout()
		}()

		buffer := make([]byte, 65535)

		for {
			n, err := src.Socket.Read(buffer)
			if err != nil {
				return
			}

			data := buffer[:n]

			_, err = dst.Socket.Write(data)
			if err != nil {
				return
			}
		}
	}

	if handshake.RequestsStatus() {
		go pipe(conn, rconn)
		go pipe(rconn, conn)

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
	p.DisconnectMessage = cfg.DisconnectMessage
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
		log.Printf("Process[%s:%s]: timed out; stopping process", p.DomainName, p.ListenTo)
		if err := p.Process.Stop(); err != nil {
			log.Printf("Process[%s:%s]: %s", p.DomainName, p.ListenTo, err)
		}
		p.hasRunningProcess = false
		return
	}
}

func (p Proxy) simulateServer(conn *net.Conn, handshake packet.SLPHandshake) {
	if handshake.RequestsLogin() {
		if err := p.sendDisconnectMessage(conn); err != nil {
			log.Println(err)
		}
		return
	}

	if err := p.makeHandshake(conn); err != nil {
		log.Println(err)
	}
}

func (p Proxy) makeHandshake(conn *net.Conn) error {
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

func (p Proxy) sendDisconnectMessage(conn *net.Conn) error {
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

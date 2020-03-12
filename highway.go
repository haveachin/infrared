package infrared

import (
	"errors"
	"fmt"
	"log"
	"time"

	mc "github.com/Tnze/go-mc/net"
	"github.com/haveachin/infrared/process"
	"github.com/haveachin/infrared/simulation"
	"github.com/haveachin/infrared/wrapper"
	proxy "github.com/jpillora/go-tcp-proxy"

	"github.com/Tnze/go-mc/net/packet"
)

// Highway is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Highway struct {
	DomainName string
	ListenTo   string
	ProxyTo    string
	Timeout    time.Duration
	Players    map[*mc.Conn]string

	proxies       map[*mc.Conn]proxy.Proxy
	server        simulation.Server
	process       process.Process
	cancelTimeout process.CancelFunc
}

// NewHighway takes a config an creates a new proxy based on it
func NewHighway(cfg Config) (*Highway, error) {
	placeholderBytes, err := cfg.MarshalPlaceholder()
	if err != nil {
		return nil, fmt.Errorf("could not mashal palceholder for %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
	}

	proc, err := processFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not load process for %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
	}

	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("could not parse timeout for %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
	}

	/* laddr, err := net.ResolveTCPAddr("tcp", cfg.ListenTo)
	if err != nil {
		return nil, err
	}

	raddr, err := net.ResolveTCPAddr("tcp", cfg.ProxyTo)
	if err != nil {
		return nil, err
	} */

	return &Highway{
		DomainName: cfg.DomainName,
		ListenTo:   cfg.ListenTo,
		ProxyTo:    cfg.ProxyTo,
		Timeout:    timeout,
		Players:    map[*mc.Conn]string{},
		proxies:    map[*mc.Conn]proxy.Proxy{},
		server: simulation.Server{
			DisconnectMessage: cfg.DisconnectMessage,
			PlaceholderPacket: wrapper.SLPResponse{
				JSONResponse: packet.String(placeholderBytes),
			}.Marshal(),
		},
		process:       proc,
		cancelTimeout: nil,
	}, nil
}

// HandleConn takes a minecraft client connection and it's initial handschake packet
// and relays all following packets to the remote connection (ProxyTo)
func (hw *Highway) HandleConn(conn *mc.Conn, handshake wrapper.SLPHandshake) error {
	if !hw.process.IsRunning() {
		defer conn.Close()
		if handshake.IsStatusRequest() {
			return hw.server.RespondToSLPStatus(*conn)
		}

		if err := hw.process.Start(); err != nil {
			log.Println(err)
			return hw.server.RespondToSLPLogin(*conn)
		}

		return hw.server.RespondToSLPLogin(*conn)
	}

	rconn, err := mc.DialMC(hw.ProxyTo)
	if err != nil {
		log.Println(err)
		return hw.server.RespondToSLP(*conn, handshake)
	}

	var pipe = func(src, dst *mc.Conn) {
		defer func() {
			if src == conn {
				return
			}

			conn.Close()
			rconn.Close()

			log.Printf("%s lost connection", hw.Players[conn])
			delete(hw.Players, conn)
			if len(hw.Players) <= 0 {
				hw.cancelTimeout = process.Timeout(hw.process, hw.Timeout)
			}
		}()

		buffer := make([]byte, 0xffff)

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

	if handshake.IsStatusRequest() {
		if err := rconn.WritePacket(handshake.Marshal()); err != nil {
			return fmt.Errorf("failed to write handshake packet to []")
		}

		pk, err := conn.ReadPacket()
		if err != nil {
			return fmt.Errorf("failed to read request packet from []")
		}

		if err := rconn.WritePacket(pk); err != nil {
			return fmt.Errorf("failed to write request packet to []")
		}
	} else if handshake.IsLoginRequest() {
		if err := rconn.WritePacket(handshake.Marshal()); err != nil {
			return fmt.Errorf("failed to write handshake packet to []")
		}

		username, err := sniffUsername(conn, rconn)
		if err != nil {
			return fmt.Errorf("failed to sniff username from []")
		}

		if hw.cancelTimeout != nil {
			log.Printf("Process[%s|%s]: timed out; stopping process", hw.DomainName, hw.ListenTo)
			hw.cancelTimeout()
			hw.cancelTimeout = nil
		}

		hw.Players[conn] = username

		go pipe(conn, rconn)
		go pipe(rconn, conn)

		return fmt.Errorf("%s connected over %s", username, hw.DomainName)
	}

	go pipe(conn, rconn)
	go pipe(rconn, conn)

	return nil
}

// ApplyConfigChange is a callback function that handles config changes
func (hw *Highway) ApplyConfigChange(cfg Config) error {
	placeholderBytes, err := cfg.MarshalPlaceholder()
	if err != nil {
		return fmt.Errorf("%s[%s] could not mashal palceholder; %s", hw.DomainName, hw.ListenTo, err)
	}

	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return fmt.Errorf("%s[%s] could not parse timeout; %s", hw.DomainName, hw.ListenTo, err)
	}

	hw.DomainName = cfg.DomainName
	hw.ListenTo = cfg.ListenTo
	hw.ProxyTo = cfg.ProxyTo
	hw.Timeout = timeout
	hw.server.PlaceholderPacket = wrapper.SLPResponse{
		JSONResponse: packet.String(placeholderBytes),
	}.Marshal()

	return nil
}

func processFromConfig(cfg Config) (process.Process, error) {
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

func sniffUsername(conn, rconn *mc.Conn) (string, error) {
	pk, err := conn.ReadPacket()
	if err != nil {
		return "", err
	}

	start, err := wrapper.ParseLoginStart(pk)
	if err != nil {
		return "", err
	}

	if err := rconn.WritePacket(pk); err != nil {
		return "", err
	}

	return string(start.Name), nil
}

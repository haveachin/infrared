package infrared

import (
	"errors"
	"fmt"
	"net"
	"time"

	mc "github.com/Tnze/go-mc/net"
	"github.com/haveachin/infrared/process"
	"github.com/haveachin/infrared/simulation"
	"github.com/haveachin/infrared/wrapper"
	"github.com/rs/zerolog/log"

	"github.com/Tnze/go-mc/net/packet"
)

type Filter func(*[]byte)

// Highway is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Highway struct {
	DomainName  string
	ListenTo    string
	ProxyTo     string
	Timeout     time.Duration
	CallbackURL string
	Players     map[*mc.Conn]string

	server        simulation.Server
	process       process.Process
	cancelTimeout func()
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

	if cfg.ProxyTo == "" {
		ip, err := net.ResolveIPAddr("127.0.0.11", cfg.Docker.ContainerName)
		if err != nil {
			return nil, fmt.Errorf("Highway[%s|%s]: Could not resolve \"%s\"", cfg.DomainName, cfg.ListenTo, cfg.Docker.ContainerName)
		}

		cfg.ProxyTo = ip.String()
	}

	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("could not parse timeout for %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
	}

	return &Highway{
		DomainName: cfg.DomainName,
		ListenTo:   cfg.ListenTo,
		ProxyTo:    cfg.ProxyTo,
		Timeout:    timeout,
		Players:    map[*mc.Conn]string{},
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
	isServerRunning := true

	ip, err := net.ResolveIPAddr("8.8.8.8", hw.ProxyTo) //127.0.0.11
	if err != nil {
		log.Err(err).Msgf("Highway[%s|%s]: Could not resolve \"%s\"", hw.DomainName, hw.ListenTo, hw.ProxyTo)
	}

	log.Debug().Msgf("Highway[%s|%s]: Resolve \"%s\" to \"%s\"", hw.DomainName, hw.ListenTo, hw.ProxyTo, ip.String())

	rconn, err := mc.DialMC(ip.String())
	if err != nil {
		isServerRunning = false
	}

	if !isServerRunning {
		defer conn.Close()
		if handshake.IsStatusRequest() {
			return hw.server.RespondToSLPStatus(*conn)
		}

		isProcessRunning, err := hw.process.IsRunning()
		if err != nil {
			log.Err(err).Msgf("Highway[%s|%s]: Could not determine if the container is running", hw.DomainName, hw.ListenTo)
			return hw.server.RespondToSLP(*conn, handshake)
		}

		if isProcessRunning {
			return hw.server.RespondToSLPLogin(*conn)
		}

		log.Info().Msgf("Process[%s|%s]: Starting", hw.DomainName, hw.ListenTo)
		if err := hw.process.Start(); err != nil {
			log.Err(err).Msgf("Process[%s|%s]: Could not start the container", hw.DomainName, hw.ListenTo)
			return hw.server.RespondToSLPLogin(*conn)
		}

		hw.startTimeout()

		return hw.server.RespondToSLPLogin(*conn)
	}

	connAddr := conn.Socket.RemoteAddr().String()

	var pipe = func(src, dst *mc.Conn, filters []Filter) {
		defer func() {
			if src == conn {
				return
			}

			conn.Close()
			rconn.Close()

			if handshake.IsStatusRequest() {
				return
			}

			log.Info().Msgf("Highway[%s|%s]: %s[%s] left the game", hw.DomainName, hw.ListenTo, hw.Players[conn], connAddr)
			delete(hw.Players, conn)

			if len(hw.Players) <= 0 {
				hw.startTimeout()
			}
		}()

		buffer := make([]byte, 0xffff)

		for {
			n, err := src.Socket.Read(buffer)
			if err != nil {
				return
			}

			data := buffer[:n]

			for _, filter := range filters {
				if filter != nil {
					filter(&data)
				}
			}

			_, err = dst.Socket.Write(data)
			if err != nil {
				return
			}
		}
	}

	if err := rconn.WritePacket(handshake.Marshal()); err != nil {
		return fmt.Errorf("failed to write handshake packet to [%s]", connAddr)
	}

	if handshake.IsStatusRequest() {
		pk, err := conn.ReadPacket()
		if err != nil {
			return fmt.Errorf("failed to read request packet from [%s]", connAddr)
		}

		if err := rconn.WritePacket(pk); err != nil {
			return fmt.Errorf("failed to write request packet to [%s]", connAddr)
		}
	} else if handshake.IsLoginRequest() {
		username, err := sniffUsername(conn, rconn)
		if err != nil {
			return fmt.Errorf("failed to sniff username from [%s]", connAddr)
		}

		hw.stopTimeout()

		hw.Players[conn] = username
		log.Info().Msgf("Highway[%s|%s]: %s[%s] joined the game", hw.DomainName, hw.ListenTo, username, connAddr)
	}

	var highPing = func(data *[]byte) {
		time.Sleep(300 * time.Millisecond)
	}

	filters := []Filter{
		highPing,
	}

	go pipe(conn, rconn, filters)
	go pipe(rconn, conn, filters)

	return nil
}

// ApplyConfigChange is a callback function that handles config changes
func (hw *Highway) ApplyConfigChange(cfg Config) {
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		log.Err(err).Msgf("Highway[%s|%s]: Could not parse timeout", hw.DomainName, hw.ListenTo)
	} else {
		hw.Timeout = timeout
	}

	placeholderBytes, err := cfg.MarshalPlaceholder()
	if err != nil {
		log.Err(err).Msgf("Highway[%s|%s]: Could not mashal palceholder", hw.DomainName, hw.ListenTo)
	} else {
		hw.server.PlaceholderPacket = wrapper.SLPResponse{
			JSONResponse: packet.String(placeholderBytes),
		}.Marshal()
	}

	proc, err := processFromConfig(cfg)
	if err != nil {
		log.Err(err).Msgf("Highway[%s|%s]: Could not update container", hw.DomainName, hw.ListenTo)
	} else {
		hw.process = proc
	}

	hw.DomainName = cfg.DomainName
	hw.ListenTo = cfg.ListenTo
	hw.ProxyTo = cfg.ProxyTo
	hw.server.DisconnectMessage = cfg.DisconnectMessage
}

func processFromConfig(cfg Config) (process.Process, error) {
	if cfg.UsesPortainer() {
		dCfg := cfg.Docker
		pCfg := dCfg.Portainer
		return process.NewPortainer(pCfg.Address, pCfg.EndpointID, dCfg.ContainerName, pCfg.Username, pCfg.Password)
	}

	if cfg.UsesDocker() {
		return process.NewDocker(cfg.Docker.ContainerName)
	}

	return nil, errors.New("no container in config")
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

func (hw *Highway) startTimeout() {
	if hw.cancelTimeout != nil {
		return
	}

	timer := time.AfterFunc(hw.Timeout, func() {
		log.Info().Msgf("Process[%s|%s]: Stopping", hw.DomainName, hw.ListenTo)
		if err := hw.process.Stop(); err != nil {
			log.Err(err).Msgf("Process[%s|%s]: Failed to stop", hw.DomainName, hw.ListenTo)
		}
	})

	hw.cancelTimeout = func() {
		timer.Stop()
		log.Info().Msgf("Process[%s|%s]: Timeout canceled", hw.DomainName, hw.ListenTo)
	}

	log.Info().Msgf("Process[%s|%s]: Timing out in %s", hw.DomainName, hw.ListenTo, hw.Timeout)
}

func (hw *Highway) stopTimeout() {
	if hw.cancelTimeout == nil {
		return
	}

	hw.cancelTimeout()
	hw.cancelTimeout = nil
}

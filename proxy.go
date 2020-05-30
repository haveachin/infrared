package infrared

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net"
	"sync"
	"time"

	"github.com/haveachin/infrared/mc"
	"github.com/haveachin/infrared/mc/protocol"
	"github.com/haveachin/infrared/mc/sim"
	"github.com/haveachin/infrared/process"
)

// Proxy is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Proxy struct {
	// ClientBoundModifiers modify traffic that is send from the server to the client
	ClientBoundModifiers []Modifier
	// ServerBoundModifiers modify traffic that is send from the client to the server
	ServerBoundModifiers []Modifier

	domainName           string
	listenTo             string
	proxyTo              string
	timeout              time.Duration
	callbackURL          string
	players              map[*mc.Conn]string

	server        sim.Server
	process       process.Process
	cancelTimeout func()

	logger zerolog.Logger
}

// NewProxy takes a config an creates a new proxy based on it
func NewProxy(cfg ProxyConfig) (*Proxy, error) {
	proxy := Proxy{
		ClientBoundModifiers: []Modifier{},
		ServerBoundModifiers: []Modifier{},
		players:       map[*mc.Conn]string{},
		cancelTimeout: nil,
	}

	if err := proxy.updateConfig(cfg); err != nil {
		return nil, err
	}

	proxy.OverrideLogger(log.Logger)

	return &proxy, nil
}

func (proxy *Proxy) OverrideLogger(logger zerolog.Logger) zerolog.Logger {
	proxy.logger = logger.With().
		Str("destinationAddress", proxy.proxyTo).
		Str("domainName", proxy.domainName).Logger()

	return proxy.logger
}

// HandleConn takes a minecraft client connection and it's initial handshake packet
// and relays all following packets to the remote connection (proxyTo)
func (proxy *Proxy) HandleConn(conn mc.Conn) error {
	connAddr := conn.RemoteAddr().String()
	logger := proxy.logger.With().Str("connection", connAddr).Logger()

	packet, err := conn.PeekPacket()
	if err != nil {
		return err
	}

	handshake, err := protocol.ParseSLPHandshake(packet)
	if err != nil {
		return err
	}

	rconn, err := mc.DialTimeout(proxy.proxyTo, time.Millisecond*500)
	if err != nil {
		defer conn.Close()
		if handshake.IsStatusRequest() {
			return proxy.server.HandleConn(conn)
		}

		isProcessRunning, err := proxy.process.IsRunning()
		if err != nil {
			logger.Err(err).Msg("Could not determine if the container is running")
			return proxy.server.HandleConn(conn)
		}

		if isProcessRunning {
			return proxy.server.HandleConn(conn)
		}

		logger.Info().Msg("Starting container")
		if err := proxy.process.Start(); err != nil {
			logger.Err(err).Msg("Could not start the container")
			return proxy.server.HandleConn(conn)
		}

		proxy.startTimeout()

		return proxy.server.HandleConn(conn)
	}

	if handshake.IsLoginRequest() {
		username, err := sniffUsername(conn, rconn)
		if err != nil {
			return err
		}

		proxy.stopTimeout()
		proxy.players[&conn] = username
		proxy.logger.Info().Msgf("%s joined the game", proxy.players[&conn])

		defer func() {
			proxy.logger.Info().Msgf("%s left the game", proxy.players[&conn])
			delete(proxy.players, &conn)

			if len(proxy.players) <= 0 {
				proxy.startTimeout()
			}
		}()
	}

	wg := sync.WaitGroup{}

	var pipe = func(src, dst mc.Conn, modifiers []Modifier) {
		defer wg.Done()

		buffer := make([]byte, 0xffff)

		for {
			n, err := src.Read(buffer)
			if err != nil {
				return
			}

			data := buffer[:n]

			for _, modifier := range modifiers {
				if modifier == nil {
					continue
				}

				modifier.Modify(src, dst, &data)
			}

			_, err = dst.Conn.Write(data)
			if err != nil {
				return
			}
		}
	}

	wg.Add(2)
	go pipe(conn, rconn, proxy.ClientBoundModifiers)
	go pipe(rconn, conn, proxy.ServerBoundModifiers)
	wg.Wait()

	conn.Close()
	rconn.Close()

	return nil
}

// updateConfig is a callback function that handles config changes
func (proxy *Proxy) updateConfig(cfg ProxyConfig) error {
	if cfg.ProxyTo == "" {
		ip, err := net.ResolveIPAddr(cfg.Process.DNSServer, cfg.Process.ContainerName)
		if err != nil {
			return err
		}

		cfg.ProxyTo = ip.String()
	}

	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return err
	}

	proc, err := process.New(cfg.Process)
	if err != nil {
		return err
	}

	if err := proxy.server.UpdateConfig(cfg.Server); err != nil {
		return err
	}

	proxy.domainName = cfg.DomainName
	proxy.listenTo = cfg.ListenTo
	proxy.proxyTo = cfg.ProxyTo
	proxy.timeout = timeout
	proxy.process = proc

	return nil
}

func (proxy *Proxy) startTimeout() {
	if proxy.cancelTimeout != nil {
		proxy.stopTimeout()
	}

	timer := time.AfterFunc(proxy.timeout, func() {
		proxy.logger.Info().Msgf("Stopping container")
		if err := proxy.process.Stop(); err != nil {
			proxy.logger.Err(err).Msg("Failed to stop the container")
		}
	})

	proxy.cancelTimeout = func() {
		timer.Stop()
		proxy.logger.Debug().Msg("Timeout canceled")
	}

	proxy.logger.Info().Msgf("Timing out in %s", proxy.timeout)
}

func (proxy *Proxy) stopTimeout() {
	if proxy.cancelTimeout == nil {
		return
	}

	proxy.cancelTimeout()
	proxy.cancelTimeout = nil
}

func (proxy Proxy) Close() {
	for conn := range proxy.players {
		if err := conn.Close(); err != nil {
			proxy.logger.Err(err)
		}
	}
}

func sniffUsername(conn, rconn mc.Conn) (string, error) {
	// Handshake
	packet, err := conn.ReadPacket()
	if err != nil {
		return "", err
	}

	if err := rconn.WritePacket(packet); err != nil {
		return "", err
	}

	// Login
	packet, err = conn.ReadPacket()
	if err != nil {
		return "", err
	}

	loginStartPacket, err := protocol.ParseClientLoginStart(packet)
	if err != nil {
		return "", err
	}

	if err := rconn.WritePacket(packet); err != nil {
		return "", err
	}

	return string(loginStartPacket.Name), nil
}

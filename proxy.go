package infrared

import (
	"github.com/haveachin/infrared/callback"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"sync"
	"time"

	"github.com/haveachin/infrared/mc"
	"github.com/haveachin/infrared/mc/protocol"
	"github.com/haveachin/infrared/mc/sim"
	"github.com/haveachin/infrared/process"
)

type playerMap struct {
	sync.RWMutex
	players map[*mc.Conn]string
}

func (p *playerMap) put(key *mc.Conn, value string) {
	p.Lock()
	defer p.Unlock()
	p.players[key] = value
}

func (p *playerMap) remove(key *mc.Conn) {
	p.Lock()
	defer p.Unlock()
	delete(p.players, key)
}

func (p *playerMap) length() int {
	p.RLock()
	defer p.RUnlock()
	return len(p.players)
}

func (p *playerMap) keys() []*mc.Conn {
	p.RLock()
	defer p.RUnlock()

	var conns []*mc.Conn

	for conn := range p.players {
		conns = append(conns, conn)
	}

	return conns
}

// Proxy is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type Proxy struct {
	// ClientBoundModifiers modify traffic that is send from the server to the client
	ClientBoundModifiers []Modifier
	// ServerBoundModifiers modify traffic that is send from the client to the server
	ServerBoundModifiers []Modifier

	domainName    string
	listenTo      string
	proxyTo       string
	ProxyProtocol bool
	timeout       time.Duration
	cancelTimeout func()
	players       playerMap

	server    sim.Server
	process   process.Process
	logWriter *callback.LogWriter

	logger        zerolog.Logger
	loggerOutputs []io.Writer
}

// NewProxy takes a config an creates a new proxy based on it
func NewProxy(cfg ProxyConfig) (*Proxy, error) {
	logWriter := &callback.LogWriter{}

	proxy := Proxy{
		ClientBoundModifiers: []Modifier{},
		ServerBoundModifiers: []Modifier{},
		ProxyProtocol: 		  cfg.ProxyProtocol,
		players:              playerMap{players: map[*mc.Conn]string{}},
		cancelTimeout:        nil,
		logWriter:            logWriter,
		loggerOutputs:        []io.Writer{logWriter},
	}

	if err := proxy.updateConfig(cfg); err != nil {
		return nil, err
	}

	proxy.overrideLogger(log.Logger)

	return &proxy, nil
}

func (proxy *Proxy) AddLoggerOutput(w io.Writer) {
	proxy.loggerOutputs = append(proxy.loggerOutputs, w)
	proxy.logger = proxy.logger.Output(io.MultiWriter(proxy.loggerOutputs...))
}

func (proxy *Proxy) overrideLogger(logger zerolog.Logger) zerolog.Logger {
	proxy.logger = logger.With().
		Str("destinationAddress", proxy.proxyTo).
		Str("domainName", proxy.domainName).Logger().
		Output(io.MultiWriter(proxy.loggerOutputs...))

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
	rconn, err := mc.DialTimeout(proxy.proxyTo, time.Millisecond*500, proxy.ProxyProtocol)
	if err != nil {
		defer conn.Close()
		if handshake.IsStatusRequest() {
			return proxy.server.HandleConn(conn)
		}

		isProcessRunning, err := proxy.process.IsRunning()
		if err != nil {
			logger.Err(err).Interface(callback.EventKey, callback.ErrorEvent).Msg("Could not determine if the container is running")
			return proxy.server.HandleConn(conn)
		}

		if isProcessRunning {
			return proxy.server.HandleConn(conn)
		}

		logger.Info().Interface(callback.EventKey, callback.ContainerStartEvent).Msg("Starting container")
		if err := proxy.process.Start(); err != nil {
			logger.Err(err).Interface(callback.EventKey, callback.ErrorEvent).Msg("Could not start the container")
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
		proxy.players.put(&conn, username)
		logger = logger.With().Str("username", username).Logger()
		logger.Info().Interface(callback.EventKey, callback.PlayerJoinEvent).Msgf("%s joined the game", username)

		defer func() {
			logger.Info().Interface(callback.EventKey, callback.PlayerLeaveEvent).Msgf("%s left the game", username)
			proxy.players.remove(&conn)

			if proxy.players.length() <= 0 {
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
		ip, err := net.ResolveIPAddr(cfg.Docker.DNSServer, cfg.Docker.ContainerName)
		if err != nil {
			return err
		}

		cfg.ProxyTo = ip.String()
	}

	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return err
	}

	proc, err := process.New(cfg.Docker)
	if err != nil {
		return err
	}

	if err := proxy.server.UpdateConfig(cfg.Server); err != nil {
		return err
	}

	logWriter, err := callback.NewLogWriter(cfg.CallbackLog)
	if err != nil {
		return err
	}

	proxy.logWriter.URL = logWriter.URL
	proxy.logWriter.Events = logWriter.Events

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
		proxy.logger.Info().Interface(callback.EventKey, callback.ContainerStopEvent).Msgf("Stopping container")
		if err := proxy.process.Stop(); err != nil {
			proxy.logger.Err(err).Interface(callback.EventKey, callback.ErrorEvent).Msg("Failed to stop the container")
		}
	})

	proxy.cancelTimeout = func() {
		timer.Stop()
		proxy.logger.Debug().Msg("Timeout canceled")
	}

	proxy.logger.Info().Interface(callback.EventKey, callback.ContainerTimeoutEvent).Msgf("Timing out in %s", proxy.timeout)
}

func (proxy *Proxy) stopTimeout() {
	if proxy.cancelTimeout == nil {
		return
	}

	proxy.cancelTimeout()
	proxy.cancelTimeout = nil
}

func (proxy *Proxy) Close() {
	for _, conn := range proxy.players.keys() {
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
package infrared

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/haveachin/infrared/mc"
	"github.com/haveachin/infrared/mc/protocol"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"io"
)

type Gate struct {
	listenTo string
	listener *mc.Listener
	proxies  map[string]*Proxy // map key is domain name
	close    chan bool
	logger   zerolog.Logger
}

func NewGate(listenTo string) (*Gate, error) {
	listener, err := mc.Listen(listenTo)
	if err != nil {
		return nil, err
	}

	gate := Gate{
		listenTo: listenTo,
		listener: listener,
		proxies:  map[string]*Proxy{},
		close:    make(chan bool, 1),
	}

	gate.overrideLogger(log.Logger)

	return &gate, nil
}

func (gate *Gate) LoggerOutput(w io.Writer) {
	gate.logger = gate.logger.Output(w)
	for _, proxy := range gate.proxies {
		proxy.LoggerOutput(w)
	}
}

func (gate *Gate) overrideLogger(logger zerolog.Logger) zerolog.Logger {
	gate.logger = logger.With().Str("gate", gate.listenTo).Logger()
	for _, proxy := range gate.proxies {
		proxy.overrideLogger(gate.logger)
	}
	return gate.logger
}

func (gate *Gate) AddProxyByViper(vpr *viper.Viper) (*Proxy, error) {
	cfg, err := LoadProxyConfig(vpr)
	if err != nil {
		return nil, err
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		return nil, err
	}

	if err := gate.AddProxy(proxy); err != nil {
		return nil, err
	}

	vpr.WatchConfig()
	vpr.OnConfigChange(gate.onConfigChange(proxy, vpr))
	return proxy, nil
}

func (gate *Gate) AddProxy(proxy *Proxy) error {
	if gate.listenTo != proxy.listenTo {
		return ErrProxyNotSupported
	}

	if _, ok := gate.proxies[proxy.domainName]; ok {
		return ErrProxySignatureAlreadyRegistered
	}

	proxy.overrideLogger(gate.logger)
	gate.proxies[proxy.domainName] = proxy

	gate.logger.Debug().
		Str("destinationAddress", proxy.proxyTo).
		Str("domainName", proxy.domainName).
		Msg("Added proxy to gate")

	return nil
}

func (gate *Gate) RemoveProxy(domainName string) {
	delete(gate.proxies, domainName)

	if len(gate.proxies) > 0 {
		return
	}

	gate.Close()
}

func (gate *Gate) ListenAndServe() error {
	gate.logger.Info().Msgf("Starting gate on %s", gate.listenTo)

	if len(gate.proxies) <= 0 {
		return ErrNoProxyInGate
	}

	for {
		conn, err := gate.listener.Accept()
		if err != nil {
			select {
			case <-gate.close:
				gate.logger.Info().Msg("Closed")
				return nil
			default:
				gate.logger.Debug().Err(err).Msg("Could not accept connection")
				continue
			}
		}

		go gate.serve(conn)
	}
}

func (gate *Gate) Close() {
	go func() {
		gate.close <- true
	}()

	if err := gate.listener.Close(); err != nil {
		gate.logger.Err(err)
	}

	for _, proxy := range gate.proxies {
		proxy.Close()
	}
}

func (gate Gate) serve(conn mc.Conn) {
	connAddr := conn.RemoteAddr().String()
	logger := gate.logger.With().Str("connection", connAddr).Logger()
	logger.Debug().Msg("Connection accepted")

	packet, err := conn.PeekPacket()
	if err != nil {
		logger.Debug().Err(err).Msg("Handshake reading failed")
		return
	}

	handshake, err := protocol.ParseSLPHandshake(packet)
	if err != nil {
		logger.Debug().Err(err).Msg("Handshake parsing failed")
		return
	}

	if handshake.IsForgeAddress() {
		logger.Debug().Msg("Connection is a forge client")
	}

	addr := handshake.ParseServerAddress()
	addrWithPort := fmt.Sprintf("%s:%d", addr, handshake.ServerPort)
	logger = logger.With().Str("requestedAddress", addrWithPort).Logger()
	proxy, ok := gate.proxies[addr]
	if !ok {
		logger.Info().Msg("Unknown address requested")
		return
	}

	if err := proxy.HandleConn(conn); err != nil {
		logger.Err(err)
	}
}

func (gate *Gate) onConfigChange(proxy *Proxy, vpr *viper.Viper) func(fsnotify.Event) {
	return func(in fsnotify.Event) {
		if in.Op != fsnotify.Write {
			return
		}

		logger := gate.logger.With().Str("path", in.Name).Logger()
		logger.Info().Msg("Configuration changed")

		cfg, err := LoadProxyConfig(vpr)
		if err != nil {
			logger.Err(err).Msg("Failed to load configuration")
			return
		}

		if cfg.ListenTo != gate.listenTo {
			logger.Err(ErrProxyNotSupported).Msg("Automatically closing this proxy now")
			vpr.OnConfigChange(nil)
			proxy.Close()
			gate.RemoveProxy(proxy.domainName)
			return
		}

		if err := gate.UpdateProxy(proxy, cfg); err != nil {
			log.Err(err)
		}
	}
}

func (gate *Gate) UpdateProxy(proxy *Proxy, cfg ProxyConfig) error {
	if cfg.DomainName == proxy.domainName {
		if err := proxy.updateConfig(cfg); err != nil {
			return err
		}
		return nil
	}

	if _, ok := gate.proxies[cfg.DomainName]; ok {
		return ErrProxySignatureAlreadyRegistered
	}

	oldDomainName := proxy.domainName

	if err := proxy.updateConfig(cfg); err != nil {
		return err
	}

	gate.proxies[proxy.domainName] = proxy
	delete(gate.proxies, oldDomainName)
	return nil
}

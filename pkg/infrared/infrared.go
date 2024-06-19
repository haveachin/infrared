package infrared

import (
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/rs/zerolog"
)

var Log = zerolog.Nop()

type Config struct {
	BindAddr            string              `yaml:"bind"`
	KeepAliveTimeout    time.Duration       `yaml:"keepAliveTimeout"`
	ServerConfigs       []ServerConfig      `yaml:"servers"`
	FiltersConfig       FiltersConfig       `yaml:"filters"`
	ProxyProtocolConfig ProxyProtocolConfig `yaml:"proxyProtocol"`
}

func NewConfig() Config {
	return Config{
		BindAddr:            "localhost:25565",
		KeepAliveTimeout:    30 * time.Second,
		ServerConfigs:       make([]ServerConfig, 0),
		FiltersConfig:       NewFilterConfig(),
		ProxyProtocolConfig: NewProxyProtocolConfig(),
	}
}

func (cfg Config) WithBindAddr(bindAddr string) Config {
	cfg.BindAddr = bindAddr
	return cfg
}

func (cfg Config) WithKeepAliveTimeout(d time.Duration) Config {
	cfg.KeepAliveTimeout = d
	return cfg
}

func (cfg Config) WithServerConfigs(sCfgs ...ServerConfig) Config {
	cfg.ServerConfigs = sCfgs
	return cfg
}

func (cfg Config) AddServerConfigs(sCfgs ...ServerConfig) Config {
	cfg.ServerConfigs = append(cfg.ServerConfigs, sCfgs...)
	return cfg
}

func (cfg Config) WithFiltersConfig(fCfg FiltersConfig) Config {
	cfg.FiltersConfig = fCfg
	return cfg
}

func (cfg Config) WithProxyProtocolConfig(ppCfg ProxyProtocolConfig) Config {
	cfg.ProxyProtocolConfig = ppCfg
	return cfg
}

type ConfigProvider interface {
	Config() (Config, error)
}

func MustProvideConfig(fn func() (Config, error)) Config {
	cfg, err := fn()
	if err != nil {
		panic(err)
	}

	return cfg
}

type (
	NewListenerFunc        func(addr string) (net.Listener, error)
	NewServerRequesterFunc func([]*Server) (ServerRequester, error)
)

type Infrared struct {
	NewListenerFunc        NewListenerFunc
	NewServerRequesterFunc NewServerRequesterFunc
	Filters                []Filterer

	cfg Config

	l      net.Listener
	filter Filter
	sr     ServerRequester

	conns map[net.Addr]*clientConn
}

func New(cfg Config) *Infrared {
	return &Infrared{
		cfg:   cfg,
		conns: make(map[net.Addr]*clientConn),
	}
}

func NewWithConfigProvider(prv ConfigProvider) *Infrared {
	cfg := MustProvideConfig(prv.Config)
	return New(cfg)
}

func (ir *Infrared) initListener() error {
	Log.Info().
		Str("bind", ir.cfg.BindAddr).
		Msg("Starting listener")

	if ir.NewListenerFunc == nil {
		ir.NewListenerFunc = func(addr string) (net.Listener, error) {
			return net.Listen("tcp", addr)
		}
	}

	if ir.cfg.ProxyProtocolConfig.Receive {
		fn := ir.NewListenerFunc
		ir.NewListenerFunc = func(addr string) (net.Listener, error) {
			l, err := fn(addr)
			if err != nil {
				return nil, err
			}

			return newProxyProtocolListener(l, ir.cfg.ProxyProtocolConfig.TrustedCIDRs)
		}
	}

	l, err := ir.NewListenerFunc(ir.cfg.BindAddr)
	if err != nil {
		return err
	}
	ir.l = l

	return nil
}

func (ir *Infrared) initServerGateway() error {
	srvs := make([]*Server, 0)
	for _, sCfg := range ir.cfg.ServerConfigs {
		srv, err := NewServer(sCfg)
		if err != nil {
			return err
		}
		srvs = append(srvs, srv)
	}

	if ir.NewServerRequesterFunc == nil {
		ir.NewServerRequesterFunc = func(s []*Server) (ServerRequester, error) {
			return NewServerGateway(srvs, nil)
		}
	}

	sr, err := ir.NewServerRequesterFunc(srvs)
	if err != nil {
		return err
	}
	ir.sr = sr

	return nil
}

func (ir *Infrared) init() error {
	if err := ir.initListener(); err != nil {
		return err
	}

	if err := ir.initServerGateway(); err != nil {
		return err
	}

	ir.filter = NewFilter(ir.cfg.FiltersConfig)
	ir.filter.filterers = append(ir.filter.filterers, ir.Filters...)

	return nil
}

func (ir *Infrared) ListenAndServe() error {
	if err := ir.init(); err != nil {
		return err
	}

	for {
		c, err := ir.l.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		} else if err != nil {
			Log.Debug().
				Err(err).
				Msg("Error accepting new connection")

			continue
		}

		go ir.handleNewConn(c)
	}
}

func (ir *Infrared) handleNewConn(c net.Conn) {
	if err := ir.filter.Filter(c); err != nil {
		Log.Debug().
			Err(err).
			Msg("Filtered connection")
		return
	}

	conn, cleanUp := newClientConn(c)
	defer func() {
		_ = conn.ForceClose()
		cleanUp()
	}()

	if err := ir.handleConn(conn); err != nil {
		Log.Debug().
			Err(err).
			Msg("Error while handling connection")
	}
}

func (ir *Infrared) handleConn(c *clientConn) error {
	if err := c.ReadPackets(&c.readPks[0], &c.readPks[1]); err != nil {
		return err
	}

	if err := c.handshake.Unmarshal(c.readPks[0]); err != nil {
		return err
	}

	reqDomain := c.handshake.ParseServerAddress()
	if strings.Contains(reqDomain, ":") {
		host, _, err := net.SplitHostPort(reqDomain)
		if err != nil {
			return err
		}
		reqDomain = host
	}
	c.reqDomain = ServerDomain(reqDomain)

	resp, err := ir.sr.RequestServer(ServerRequest{
		ClientAddr:      c.RemoteAddr(),
		Domain:          c.reqDomain,
		IsLogin:         c.handshake.IsLoginRequest(),
		ProtocolVersion: protocol.Version(c.handshake.ProtocolVersion),
		ReadPackets:     c.readPks,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrServerNotReachable) && c.handshake.IsLoginRequest():
			return ir.handleLoginDisconnect(c, resp)
		case errors.Is(err, ErrServerNotFound):
			return ir.handleServerNotFound(c)
		default:
			return err
		}
	}

	if c.handshake.IsStatusRequest() {
		return handleStatus(c, resp)
	}

	return ir.handleLogin(c, resp)
}

func handleStatus(c *clientConn, resp ServerResponse) error {
	if err := c.WritePacket(resp.Packet); err != nil {
		return err
	}

	pingPk := c.readPks[0]
	if err := c.ReadPacket(&pingPk); err != nil {
		return err
	}

	if err := c.WritePacket(pingPk); err != nil {
		return err
	}

	return nil
}

func (ir *Infrared) handleLoginDisconnect(c *clientConn, resp ServerResponse) error {
	return c.WritePacket(resp.Packet)
}

func (ir *Infrared) handleServerNotFound(c *clientConn) error {
	return nil
}

func (ir *Infrared) handleLogin(c *clientConn, resp ServerResponse) error {
	hsVersion := protocol.Version(c.handshake.ProtocolVersion)
	if err := c.loginStart.Unmarshal(c.readPks[1], hsVersion); err != nil {
		return err
	}

	c.timeout = ir.cfg.KeepAliveTimeout

	return ir.handlePipe(c, resp)
}

func (ir *Infrared) handlePipe(c *clientConn, resp ServerResponse) error {
	rc := resp.ServerConn
	defer rc.Close()

	if resp.SendProxyProtocol {
		if err := writeProxyProtocolHeader(c.RemoteAddr(), rc); err != nil {
			return err
		}
	}

	if err := rc.WritePackets(c.readPks[0], c.readPks[1]); err != nil {
		return err
	}

	rcClosedChan := make(chan struct{})
	cClosedChan := make(chan struct{})

	c.timeout = ir.cfg.KeepAliveTimeout
	rc.timeout = ir.cfg.KeepAliveTimeout
	ir.conns[c.RemoteAddr()] = c

	go ir.pipe(rc, c, cClosedChan)
	go ir.pipe(c, rc, rcClosedChan)

	var waitChan chan struct{}
	select {
	case <-cClosedChan:
		rc.Close()
		waitChan = rcClosedChan
	case <-rcClosedChan:
		c.ForceClose()
		waitChan = cClosedChan
	}
	<-waitChan
	delete(ir.conns, c.RemoteAddr())

	return nil
}

func (ir *Infrared) pipe(dst io.WriteCloser, src io.ReadCloser, srcClosedChan chan struct{}) {
	if _, err := io.Copy(dst, src); err != nil && !errors.Is(err, io.EOF) {
		Log.Debug().
			Err(err).
			Msg("Connection closed unexpectedly")
	}

	srcClosedChan <- struct{}{}
}

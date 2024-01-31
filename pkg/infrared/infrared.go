package infrared

import (
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/pires/go-proxyproto"
)

type Config struct {
	BindAddr         string         `yaml:"bind"`
	ServerConfigs    []ServerConfig `yaml:"servers"`
	FiltersConfig    FiltersConfig  `yaml:"filters"`
	KeepAliveTimeout time.Duration  `yaml:"keepAliveTimeout"`
}

type ConfigFunc func(cfg *Config)

func WithBindAddr(bindAddr string) ConfigFunc {
	return func(cfg *Config) {
		cfg.BindAddr = bindAddr
	}
}

func AddServerConfig(fns ...ServerConfigFunc) ConfigFunc {
	return func(cfg *Config) {
		var sCfg ServerConfig
		for _, fn := range fns {
			fn(&sCfg)
		}
		cfg.ServerConfigs = append(cfg.ServerConfigs, sCfg)
	}
}

func WithKeepAliveTimeout(d time.Duration) ConfigFunc {
	return func(cfg *Config) {
		cfg.KeepAliveTimeout = d
	}
}

func DefaultConfig() Config {
	return Config{
		BindAddr:         ":25565",
		KeepAliveTimeout: 30 * time.Second,
	}
}

type ConfigProvider interface {
	Config() (Config, error)
}

func MustConfig(fn func() (Config, error)) Config {
	cfg, err := fn()
	if err != nil {
		panic(err)
	}

	return cfg
}

type Infrared struct {
	cfg Config

	l       net.Listener
	sg      serverGateway
	filter  Filter
	bufPool sync.Pool
	conns   map[net.Addr]*conn
}

func New(fns ...ConfigFunc) *Infrared {
	cfg := DefaultConfig()
	for _, fn := range fns {
		fn(&cfg)
	}

	return NewWithConfig(cfg)
}

func NewWithConfigProvider(prv ConfigProvider) *Infrared {
	cfg := MustConfig(prv.Config)
	return NewWithConfig(cfg)
}

func NewWithConfig(cfg Config) *Infrared {
	return &Infrared{
		cfg: cfg,
		bufPool: sync.Pool{
			New: func() any {
				b := make([]byte, 1<<15)
				return &b
			},
		},
		conns: make(map[net.Addr]*conn),
	}
}

func (ir *Infrared) init() error {
	log.Printf("Listening on %s", ir.cfg.BindAddr)
	l, err := net.Listen("tcp", ir.cfg.BindAddr)
	if err != nil {
		return err
	}
	ir.l = l

	srvs := make([]*Server, 0)
	for _, sCfg := range ir.cfg.ServerConfigs {
		srv, err := NewServer(WithServerConfig(sCfg))
		if err != nil {
			return err
		}
		srvs = append(srvs, srv)
	}

	ir.filter = NewFilter(WithFilterConfig(ir.cfg.FiltersConfig))
	ir.sg = serverGateway{
		servers: srvs,
	}

	return nil
}

func (ir *Infrared) ListenAndServe() error {
	if err := ir.init(); err != nil {
		return err
	}

	sgInChan := make(chan ServerRequest)
	defer close(sgInChan)

	errChan := make(chan error)

	go func() {
		if err := ir.sg.listenAndServe(sgInChan); err != nil {
			errChan <- err
		}
	}()

	go func() {
		if err := ir.listenAndServe(sgInChan); err != nil {
			errChan <- err
		}
	}()

	return <-errChan
}

func (ir *Infrared) listenAndServe(srvReqChan chan<- ServerRequest) error {
	for {
		c, err := ir.l.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		} else if err != nil {
			log.Printf("Error accepting new conn: %s", err)
			continue
		}

		go ir.handleNewConn(c, srvReqChan)
	}
}

func (ir *Infrared) handleNewConn(c net.Conn, srvReqChan chan<- ServerRequest) {
	if err := ir.filter.Filter(c); err != nil {
		//log.Printf("Filtered: %s", err)
		return
	}

	conn := newConn(c)
	defer func() {
		conn.ForceClose()
		connPool.Put(conn)
	}()

	conn.srvReqChan = srvReqChan

	if err := ir.handleConn(conn); err != nil {
		log.Println(err)
	}
}

func (ir *Infrared) handleConn(c *conn) error {
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

	respChan := make(chan ServerRequestResponse)
	c.srvReqChan <- ServerRequest{
		Domain:          c.reqDomain,
		IsLogin:         c.handshake.IsLoginRequest(),
		ProtocolVersion: protocol.Version(c.handshake.ProtocolVersion),
		ReadPks:         c.readPks,
		ResponseChan:    respChan,
	}

	resp := <-respChan
	if resp.Err != nil {
		return resp.Err
	}

	if c.handshake.IsStatusRequest() {
		return handleStatus(c, resp)
	}

	return ir.handleLogin(c, resp)
}

func handleStatus(c *conn, resp ServerRequestResponse) error {
	if err := c.WritePacket(resp.StatusResponse); err != nil {
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

func (ir *Infrared) handleLogin(c *conn, resp ServerRequestResponse) error {
	hsVersion := protocol.Version(c.handshake.ProtocolVersion)
	if err := c.loginStart.Unmarshal(c.readPks[1], hsVersion); err != nil {
		return err
	}

	c.timeout = ir.cfg.KeepAliveTimeout

	return ir.handlePipe(c, resp)
}

func (ir *Infrared) handlePipe(c *conn, resp ServerRequestResponse) error {
	rc := resp.ServerConn
	defer rc.ForceClose()

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

	go ir.copy(rc, c, cClosedChan)
	go ir.copy(c, rc, rcClosedChan)

	var waitChan chan struct{}
	select {
	case <-cClosedChan:
		rc.ForceClose()
		waitChan = rcClosedChan
	case <-rcClosedChan:
		c.ForceClose()
		waitChan = cClosedChan
	}
	<-waitChan
	delete(ir.conns, c.RemoteAddr())

	return nil
}

func (ir *Infrared) copy(dst io.WriteCloser, src io.ReadCloser, srcClosedChan chan struct{}) {
	b := ir.bufPool.Get().(*[]byte)
	defer ir.bufPool.Put(b)

	io.CopyBuffer(dst, src, *b)
	srcClosedChan <- struct{}{}
}

func writeProxyProtocolHeader(addr net.Addr, rc net.Conn) error {
	rcAddr := rc.RemoteAddr()
	tcpAddr := rcAddr.(*net.TCPAddr)

	tp := proxyproto.TCPv4
	if tcpAddr.IP.To4() == nil {
		tp = proxyproto.TCPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        addr,
		DestinationAddr:   rcAddr,
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}

	return nil
}

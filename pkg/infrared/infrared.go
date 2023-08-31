package infrared

import (
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
	ListenerConfigs  map[ListenerID]ListenerConfig `yaml:"listeners"`
	ServerConfigs    map[ServerID]ServerConfig     `yaml:"servers"`
	KeepAliveTimeout time.Duration                 `yaml:"keep_alive_timeout"`
}

type ConfigFunc func(cfg *Config)

func AddListenerConfig(id ListenerID, fns ...ListenerConfigFunc) ConfigFunc {
	return func(cfg *Config) {
		var lCfg ListenerConfig
		for _, fn := range fns {
			fn(&lCfg)
		}
		cfg.ListenerConfigs[id] = lCfg
	}
}

func AddServerConfig(id ServerID, fns ...ServerConfigFunc) ConfigFunc {
	return func(cfg *Config) {
		var sCfg ServerConfig
		for _, fn := range fns {
			fn(&sCfg)
		}
		cfg.ServerConfigs[id] = sCfg
	}
}

func WithKeepAliveTimeout(d time.Duration) ConfigFunc {
	return func(cfg *Config) {
		cfg.KeepAliveTimeout = d
	}
}

func DefaultConfig() Config {
	return Config{
		KeepAliveTimeout: 30 * time.Second,
	}
}

type Infrared struct {
	cfg Config

	listeners []*Listener
	srvs      []*Server
	bufPool   sync.Pool
	conns     map[net.Addr]*conn
	mu        sync.Mutex
}

func New(fns ...ConfigFunc) *Infrared {
	cfg := DefaultConfig()
	for _, fn := range fns {
		fn(&cfg)
	}

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
	for _, lCfg := range ir.cfg.ListenerConfigs {
		l, err := NewListener(func(cfg *ListenerConfig) {
			*cfg = lCfg
		})
		if err != nil {
			return err
		}
		ir.listeners = append(ir.listeners, l)
	}

	for _, sCfg := range ir.cfg.ServerConfigs {
		srv, err := NewServer(WithServerConfig(sCfg))
		if err != nil {
			return err
		}

		ir.srvs = append(ir.srvs, srv)
	}

	return nil
}

func (ir *Infrared) ListenAndServe() error {
	if err := ir.init(); err != nil {
		return err
	}

	sgInChan := make(chan ServerRequest)
	for _, l := range ir.listeners {
		go func(l net.Listener) {
			for {
				c, err := l.Accept()
				if err != nil {
					log.Println(err)
					continue
				}

				go func(c net.Conn) {
					conn := newConn(c)
					defer func() {
						conn.ForceClose()
						connPool.Put(conn)
					}()

					conn.srvReqChan = sgInChan

					if err := ir.handleConn(conn); err != nil {
						log.Println(err)
					}
				}(c)
			}
		}(l)
	}

	sg := serverGateway{
		Servers:     ir.srvs,
		requestChan: sgInChan,
	}
	return sg.listenAndServe()
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
		return ir.handleStatus(c, resp)
	}

	return ir.handleLogin(c, resp)
}

func (ir *Infrared) handleStatus(c *conn, resp ServerRequestResponse) error {
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

	return ir.handlePipe(c, resp.ServerConn)
}

func (ir *Infrared) handlePipe(c, rc *conn) error {
	defer rc.ForceClose()

	if resp.UseProxyProtocol {
		header := &proxyproto.Header{
			Version:           2,
			Command:           proxyproto.PROXY,
			TransportProtocol: proxyproto.TCPv4,
			SourceAddr:        c.RemoteAddr(),
			DestinationAddr:   rc.RemoteAddr(),
		}

		if _, err := header.WriteTo(rc); err != nil {
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
	ir.addConn(c)

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
	ir.removeConn(c)

	return nil
}

func (ir *Infrared) copy(dst io.WriteCloser, src io.ReadCloser, srcClosedChan chan struct{}) {
	b := ir.bufPool.Get().(*[]byte)
	defer ir.bufPool.Put(b)

	io.CopyBuffer(dst, src, *b)
	srcClosedChan <- struct{}{}
}

func (ir *Infrared) addConn(c *conn) {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.conns[c.RemoteAddr()] = c
}

func (ir *Infrared) removeConn(c *conn) {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	delete(ir.conns, c.RemoteAddr())
}

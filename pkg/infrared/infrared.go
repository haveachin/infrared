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
	Bind          string
	ServerConfigs []ServerConfig
}

type ConfigFunc func(cfg *Config)

func WithBind(bind string) ConfigFunc {
	return func(cfg *Config) {
		cfg.Bind = bind
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

type Infrared struct {
	cfg Config

	l       net.Listener
	srvs    []*Server
	bufPool sync.Pool
}

func New(fns ...ConfigFunc) *Infrared {
	var cfg Config
	for _, fn := range fns {
		fn(&cfg)
	}

	return &Infrared{
		cfg: cfg,
		bufPool: sync.Pool{
			New: func() any {
				b := make([]byte, 1<<15)
				return &b
			},
		},
	}
}

func (ir *Infrared) init() error {
	l, err := net.Listen("tcp", ir.cfg.Bind)
	if err != nil {
		return err
	}
	ir.l = l

	for _, sCfg := range ir.cfg.ServerConfigs {
		ir.srvs = append(ir.srvs, NewServer(WithServerConfig(sCfg)))
	}

	return nil
}

func (ir *Infrared) ListenAndServe() error {
	if err := ir.init(); err != nil {
		return err
	}

	sgInChan := make(chan ServerRequest)
	go func() {
		for {
			c, err := ir.l.Accept()
			if err != nil {
				// TODO: Handle Listener closed
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
	}()

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

	c.timeout = time.Second * 30

	return ir.handlePipe(c, resp)
}

func (ir *Infrared) handlePipe(c *conn, resp ServerRequestResponse) error {
	rc := resp.ServerConn
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

	return nil
}

func (ir *Infrared) copy(dst io.WriteCloser, src io.ReadCloser, srcClosedChan chan struct{}) {
	b := ir.bufPool.Get().(*[]byte)
	defer ir.bufPool.Put(b)

	io.CopyBuffer(dst, src, *b)
	srcClosedChan <- struct{}{}
}

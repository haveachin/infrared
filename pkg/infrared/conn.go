package infrared

import (
	"bufio"
	"io"
	"net"
	"sync"
	"time"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/pkg/infrared/protocol/login"
)

var cliConnPool = sync.Pool{
	New: func() any {
		return &clientConn{
			readPks: [2]protocol.Packet{},
		}
	},
}

type conn struct {
	net.Conn

	r       *bufio.Reader
	w       io.Writer
	timeout time.Duration
}

func newConn(c net.Conn) conn {
	if c == nil {
		panic("c cannot be nil")
	}

	return conn{
		Conn:    c,
		r:       bufio.NewReader(c),
		w:       c,
		timeout: time.Second * 10,
	}
}

func (c *conn) Read(b []byte) (int, error) {
	if err := c.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.r.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.w.Write(b)
}

func (c *conn) ReadPacket(pk *protocol.Packet) error {
	_, err := pk.ReadFrom(c.r)
	return err
}

func (c *conn) ReadPackets(pks ...*protocol.Packet) error {
	for i := 0; i < len(pks); i++ {
		if err := c.ReadPacket(pks[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *conn) WritePacket(pk protocol.Packet) error {
	_, err := pk.WriteTo(c.w)
	return err
}

func (c *conn) WritePackets(pks ...protocol.Packet) error {
	for _, pk := range pks {
		if err := c.WritePacket(pk); err != nil {
			return err
		}
	}
	return nil
}

func (c *conn) ForceClose() error {
	if conn, ok := c.Conn.(*net.TCPConn); ok {
		if err := conn.SetLinger(0); err != nil {
			return err
		}
	}
	return c.Close()
}

func (c *conn) Close() error {
	return c.Conn.Close()
}

type ServerConn struct {
	conn
}

func NewServerConn(c net.Conn) *ServerConn {
	return &ServerConn{
		conn: newConn(c),
	}
}

type clientConn struct {
	conn

	readPks    [2]protocol.Packet
	handshake  handshaking.ServerBoundHandshake
	loginStart login.ServerBoundLoginStart
	reqDomain  ServerDomain
}

func newClientConn(c net.Conn) (*clientConn, func()) {
	conn, ok := cliConnPool.Get().(*clientConn)
	if !ok {
		panic("connPool contains other implementations of net.Conn")
	}

	conn.conn = newConn(c)
	conn.reqDomain = ""
	return conn, func() {
		cliConnPool.Put(conn)
	}
}

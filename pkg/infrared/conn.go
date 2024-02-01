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

var connPool = sync.Pool{
	New: func() any {
		return &Conn{
			readPks: [2]protocol.Packet{},
		}
	},
}

type Conn struct {
	net.Conn

	r          *bufio.Reader
	w          io.Writer
	timeout    time.Duration
	readPks    [2]protocol.Packet
	handshake  handshaking.ServerBoundHandshake
	loginStart login.ServerBoundLoginStart
	reqDomain  ServerDomain
	srvReqChan chan<- ServerRequest
}

func newConn(c net.Conn) *Conn {
	if c == nil {
		panic("c cannot be nil")
	}

	conn, ok := connPool.Get().(*Conn)
	if !ok {
		panic("connPool contains other implementations of net.Conn")
	}

	conn.Conn = c
	conn.r = bufio.NewReader(c)
	conn.w = c
	conn.reqDomain = ""
	conn.timeout = time.Second * 10
	return conn
}

func (c *Conn) Read(b []byte) (int, error) {
	if err := c.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.r.Read(b)
}

func (c *Conn) Write(b []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.w.Write(b)
}

func (c *Conn) ReadPacket(pk *protocol.Packet) error {
	_, err := pk.ReadFrom(c.r)
	return err
}

func (c *Conn) ReadPackets(pks ...*protocol.Packet) error {
	for i := 0; i < len(pks); i++ {
		if err := c.ReadPacket(pks[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) WritePacket(pk protocol.Packet) error {
	_, err := pk.WriteTo(c.w)
	return err
}

func (c *Conn) WritePackets(pks ...protocol.Packet) error {
	for _, pk := range pks {
		if err := c.WritePacket(pk); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) ForceClose() error {
	if conn, ok := c.Conn.(*net.TCPConn); ok {
		if err := conn.SetLinger(0); err != nil {
			return err
		}
	}
	return c.Close()
}

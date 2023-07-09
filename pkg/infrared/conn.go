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
		return &conn{
			readPks: [2]protocol.Packet{},
			timeout: time.Second * 10,
		}
	},
}

type conn struct {
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

func newConn(c net.Conn) *conn {
	conn := connPool.Get().(*conn)
	conn.Conn = c
	conn.r = bufio.NewReader(c)
	conn.w = c
	conn.reqDomain = ""
	return conn
}

func (c *conn) Read(b []byte) (int, error) {
	c.SetReadDeadline(time.Now().Add(c.timeout))
	return c.r.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	c.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.w.Write(b)
}

// ReadPacket read a Packet from Conn.
func (c *conn) ReadPacket(pk *protocol.Packet) error {
	_, err := pk.ReadFrom(c.r)
	return err
}

// ReadPacket read a Packet from Conn.
func (c *conn) ReadPackets(pks []*protocol.Packet) error {
	for i := 0; i < len(pks); i++ {
		if err := c.ReadPacket(pks[i]); err != nil {
			return err
		}
	}
	return nil
}

// WritePacket write a Packet to Conn.
func (c *conn) WritePacket(pk protocol.Packet) error {
	_, err := pk.WriteTo(c.w)
	return err
}

// WritePackets writes Packets to Conn.
func (c *conn) WritePackets(pks ...protocol.Packet) error {
	for _, pk := range pks {
		if err := c.WritePacket(pk); err != nil {
			return err
		}
	}
	return nil
}

package infrared

import (
	"log"
	"net"
	"strings"

	"github.com/haveachin/infrared/protocol/handshaking"
	"github.com/haveachin/infrared/protocol/login"
	"github.com/pires/go-proxyproto"
)

// Connection Processing Node
type CPN struct{}

func (cpn *CPN) Start(cpnChan <-chan ProcessingConn, srvChan chan<- ProcessingConn) {
	for {
		c, ok := <-cpnChan
		if !ok {
			break
		}
		log.Printf("[cpn|i] %s\n", c.RemoteAddr())

		if err := process(&c); err != nil {
			log.Printf("[cpn|x] %s err=%s\n", c.RemoteAddr(), err)
			c.Close()
			continue
		}
		srvChan <- c
	}
}

func process(c *ProcessingConn) error {
	if c.proxyProtocol {
		header, err := proxyproto.Read(c.Reader())
		if err != nil {
			return err
		}
		c.remoteAddr = header.SourceAddr
	}

	pks, err := c.ReadPackets(2)
	if err != nil {
		return err
	}
	c.readPks = pks

	hs, err := handshaking.UnmarshalServerBoundHandshake(pks[0])
	if err != nil {
		return err
	}
	c.handshake = hs

	c.srvHost = hs.ParseServerAddress()
	if strings.Contains(c.srvHost, ":") {
		c.srvHost, _, err = net.SplitHostPort(hs.ParseServerAddress())
		if err != nil {
			return err
		}
	}

	if c.realIP {
		addr, _, _, err := hs.ParseRealIP()
		if err != nil {
			return err
		}
		c.remoteAddr = addr
	}

	if hs.IsStatusRequest() {
		return nil
	}

	ls, err := login.UnmarshalServerBoundLoginStart(pks[1])
	if err != nil {
		return err
	}
	c.username = string(ls.Name)

	return nil
}

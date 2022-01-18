package java

import (
	"net"
	"strings"
	"time"

	"github.com/haveachin/infrared"
	"github.com/haveachin/infrared/java/protocol/handshaking"
	"github.com/haveachin/infrared/java/protocol/login"
	"github.com/pires/go-proxyproto"
)

type ConnProcessor struct{}

func (cp ConnProcessor) ProcessConn(c net.Conn) (infrared.ProcessedConn, error) {
	// TODO: Add client timeout config setting
	pc := ProcessedConn{
		Conn:       *c.(*Conn),
		remoteAddr: c.RemoteAddr(),
	}

	pc.SetReadDeadline(time.Now().Add(time.Second))
	if pc.proxyProtocol {
		header, err := proxyproto.Read(pc.r)
		if err != nil {
			return nil, err
		}
		pc.remoteAddr = header.SourceAddr
	}

	pks, err := pc.ReadPackets(2)
	if err != nil {
		return nil, err
	}
	pc.SetReadDeadline(time.Time{})
	pc.readPks = pks

	hs, err := handshaking.UnmarshalServerBoundHandshake(pks[0])
	if err != nil {
		return nil, err
	}
	pc.handshake = hs

	pc.serverAddr = hs.ParseServerAddress()
	if strings.Contains(pc.serverAddr, ":") {
		pc.serverAddr, _, err = net.SplitHostPort(pc.serverAddr)
		if err != nil {
			return nil, err
		}
	}

	if pc.realIP {
		addr, _, _, err := hs.ParseRealIP()
		if err != nil {
			return nil, err
		}
		pc.remoteAddr = addr
	}

	if hs.IsStatusRequest() {
		return &pc, nil
	}

	ls, err := login.UnmarshalServerBoundLoginStart(pks[1])
	if err != nil {
		return nil, err
	}
	pc.username = string(ls.Name)

	return &pc, nil
}

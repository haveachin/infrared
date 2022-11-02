package java

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/handshaking"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
	"github.com/pires/go-proxyproto"
)

type InfraredConnProcessor struct {
	ConnProcessor
	mu sync.RWMutex
}

func (cp *InfraredConnProcessor) ClientTimeout() time.Duration {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.ConnProcessor.ClientTimeout
}

type ConnProcessor struct {
	ClientTimeout time.Duration
}

func (cp ConnProcessor) ProcessConn(c net.Conn) (net.Conn, error) {
	pc := ProcessedConn{
		Conn:       *c.(*Conn),
		remoteAddr: c.RemoteAddr(),
		readPks:    make([]protocol.Packet, 0, 2),
	}

	if pc.proxyProtocol {
		header, err := proxyproto.Read(pc.r)
		if err != nil {
			return nil, err
		}
		pc.remoteAddr = header.SourceAddr
	}

	pk, err := pc.ReadPacket(handshaking.MaxSizeServerBoundHandshake)
	if err != nil {
		return nil, err
	}
	pc.readPks = append(pc.readPks, pk)

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
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
		pk, err := pc.ReadPacket(status.MaxSizeServerBoundPingRequest)
		if err != nil {
			return nil, err
		}
		pc.readPks = append(pc.readPks, pk)
		return &pc, nil
	}

	pk, err = pc.ReadPacket(login.MaxSizeServerBoundLoginStart)
	if err != nil {
		return nil, err
	}
	pc.readPks = append(pc.readPks, pk)

	ls, err := login.UnmarshalServerBoundLoginStart(pk, int32(hs.ProtocolVersion))
	if err != nil {
		return nil, err
	}
	pc.username = string(ls.Name)

	return &pc, nil
}

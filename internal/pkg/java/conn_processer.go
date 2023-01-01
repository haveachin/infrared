package java

import (
	"net"
	"strings"
	"time"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/handshaking"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
)

type ConnProcessor struct {
	clientTimeout time.Duration
}

func (cp ConnProcessor) ClientTimeout() time.Duration {
	return cp.clientTimeout
}

func (cp ConnProcessor) ProcessConn(c net.Conn) (net.Conn, error) {
	player := &Player{
		Conn:       *c.(*Conn),
		remoteAddr: c.RemoteAddr(),
		readPks:    make([]protocol.Packet, 0, 2),
	}

	pk, err := player.ReadPacket(handshaking.MaxSizeServerBoundHandshake)
	if err != nil {
		return nil, err
	}
	player.readPks = append(player.readPks, pk)

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		return nil, err
	}
	player.handshake = hs

	player.serverAddr = hs.ParseServerAddress()
	if strings.Contains(player.serverAddr, ":") {
		player.serverAddr, _, err = net.SplitHostPort(player.serverAddr)
		if err != nil {
			return nil, err
		}
	}

	if player.realIP {
		addr, _, _, err := hs.ParseRealIP()
		if err != nil {
			return nil, err
		}
		player.remoteAddr = addr
	}

	if hs.IsStatusRequest() {
		pk, err := player.ReadPacket(status.MaxSizeServerBoundPingRequest)
		if err != nil {
			return nil, err
		}
		player.readPks = append(player.readPks, pk)
		return player, nil
	}

	pk, err = player.ReadPacket(login.MaxSizeServerBoundLoginStart)
	if err != nil {
		return nil, err
	}
	player.readPks = append(player.readPks, pk)

	ls, err := login.UnmarshalServerBoundLoginStart(pk, int32(hs.ProtocolVersion))
	if err != nil {
		return nil, err
	}
	player.username = string(ls.Name)

	return player, nil
}

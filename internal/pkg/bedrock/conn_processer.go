package bedrock

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/packet"
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
	player := Player{
		Conn:       *c.(*Conn),
		remoteAddr: c.RemoteAddr(),
	}

	if player.proxyProtocol {
		header, err := proxyproto.Read(bufio.NewReader(c))
		if err != nil {
			return nil, err
		}
		player.remoteAddr = header.SourceAddr
	}

	pksData, err := player.ReadPackets()
	if err != nil {
		return nil, err
	}

	if len(pksData) != 1 {
		return nil, fmt.Errorf("invalid amount of packets received: expected 1; got %d", len(pksData))
	}

	if err := handlePacket(&player, pksData[0]); err != nil {
		return nil, err
	}

	if strings.Contains(player.requestedAddr, ":") {
		player.requestedAddr, _, err = net.SplitHostPort(player.requestedAddr)
		if err != nil {
			return nil, err
		}
	}

	return &player, nil
}

func handlePacket(player *Player, pkData packet.Data) error {
	switch pkData.Header.PacketID {
	case packet.IDRequestNetworkSettings:
		return handleNetworkSettings(player, pkData)
	case packet.IDLogin:
		return handleLogin(player, pkData)
	default:
		return fmt.Errorf("unknown packet with ID 0x%x", pkData.Header.PacketID)
	}
}

func handleNetworkSettings(player *Player, pkData packet.Data) error {
	var requestNetworkSettingsPk packet.RequestNetworkSettings
	if err := pkData.Decode(&requestNetworkSettingsPk); err != nil {
		return err
	}
	player.requestNetworkSettingsPkData = &pkData
	player.version = requestNetworkSettingsPk.ClientProtocol

	if err := player.WritePacket(&packet.NetworkSettings{
		CompressionThreshold: 512,
		CompressionAlgorithm: player.compression,
	}); err != nil {
		return err
	}
	player.EnableCompression(player.compression)

	pksData, err := player.ReadPackets()
	if err != nil {
		return err
	}

	if len(pksData) != 1 {
		return fmt.Errorf("invalid amount of packets received: expected 1; got %d", len(pksData))
	}

	return handleLogin(player, pksData[0])
}

func handleLogin(player *Player, pkData packet.Data) error {
	var loginPk packet.Login
	if err := pkData.Decode(&loginPk); err != nil {
		return err
	}
	player.loginPkData = pkData

	iData, cData, err := login.Parse(loginPk.ConnectionRequest)
	if err != nil {
		return err
	}
	player.username = iData.DisplayName
	player.requestedAddr = cData.ServerAddress
	return nil
}

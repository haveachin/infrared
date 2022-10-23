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
	pc := ProcessedConn{
		Conn:       *c.(*Conn),
		remoteAddr: c.RemoteAddr(),
	}

	if pc.proxyProtocol {
		header, err := proxyproto.Read(bufio.NewReader(c))
		if err != nil {
			return nil, err
		}
		pc.remoteAddr = header.SourceAddr
	}

	pksData, err := pc.ReadPackets()
	if err != nil {
		return nil, err
	}

	if len(pksData) != 1 {
		return nil, fmt.Errorf("invalid amount of packets received: expected 1; got %d", len(pksData))
	}

	if err := handlePacket(&pc, pksData[0]); err != nil {
		return nil, err
	}

	if strings.Contains(pc.serverAddr, ":") {
		pc.serverAddr, _, err = net.SplitHostPort(pc.serverAddr)
		if err != nil {
			return nil, err
		}
	}

	return &pc, nil
}

func handlePacket(pc *ProcessedConn, pkData packet.Data) error {
	switch pkData.Header.PacketID {
	case packet.IDRequestNetworkSettings:
		return handleNetworkSettings(pc, pkData)
	case packet.IDLogin:
		return handleLogin(pc, pkData)
	default:
		return fmt.Errorf("unknown packet with ID 0x%x", pkData.Header.PacketID)
	}
}

func handleNetworkSettings(pc *ProcessedConn, pkData packet.Data) error {
	var requestNetworkSettingsPk packet.RequestNetworkSettings
	if err := pkData.Decode(&requestNetworkSettingsPk); err != nil {
		return err
	}
	pc.requestNetworkSettingsPkData = &pkData
	pc.version = requestNetworkSettingsPk.ClientProtocol

	if err := pc.WritePacket(&packet.NetworkSettings{
		CompressionThreshold: 512,
		CompressionAlgorithm: pc.compression,
	}); err != nil {
		return err
	}
	pc.EnableCompression(pc.compression)

	pksData, err := pc.ReadPackets()
	if err != nil {
		return err
	}

	if len(pksData) != 1 {
		return fmt.Errorf("invalid amount of packets received: expected 1; got %d", len(pksData))
	}

	return handleLogin(pc, pksData[0])
}

func handleLogin(pc *ProcessedConn, pkData packet.Data) error {
	var loginPk packet.Login
	if err := pkData.Decode(&loginPk); err != nil {
		return err
	}
	pc.loginPkData = pkData

	iData, cData, err := login.Parse(loginPk.ConnectionRequest)
	if err != nil {
		return err
	}
	pc.username = iData.DisplayName
	pc.serverAddr = cData.ServerAddress
	return nil
}

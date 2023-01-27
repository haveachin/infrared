package handshaking

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
)

const (
	MaxSizeServerBoundHandshake = 1 + 2 + 255*4 + 2 + 1

	IDServerBoundHandshake byte = 0x00

	StateStatusServerBoundHandshake = protocol.Byte(1)
	StateLoginServerBoundHandshake  = protocol.Byte(2)

	SeparatorForge  = "\x00"
	SeparatorRealIP = "///"
)

type ServerBoundHandshake struct {
	ProtocolVersion protocol.VarInt
	ServerAddress   protocol.String
	ServerPort      protocol.UnsignedShort
	NextState       protocol.Byte
}

func (pk ServerBoundHandshake) Marshal() protocol.Packet {
	return protocol.MarshalPacket(
		IDServerBoundHandshake,
		pk.ProtocolVersion,
		pk.ServerAddress,
		pk.ServerPort,
		pk.NextState,
	)
}

func UnmarshalServerBoundHandshake(packet protocol.Packet) (ServerBoundHandshake, error) {
	var pk ServerBoundHandshake

	if packet.ID != IDServerBoundHandshake {
		return pk, protocol.ErrInvalidPacketID
	}

	if err := packet.Scan(
		&pk.ProtocolVersion,
		&pk.ServerAddress,
		&pk.ServerPort,
		&pk.NextState,
	); err != nil {
		return pk, err
	}

	return pk, nil
}

func (pk *ServerBoundHandshake) SetServerAddress(addr string) {
	oldAddr := pk.ParseServerAddress()
	newAddr := strings.Replace(string(pk.ServerAddress), oldAddr, addr, 1)
	pk.ServerAddress = protocol.String(newAddr)
}

func (pk ServerBoundHandshake) IsStatusRequest() bool {
	return pk.NextState == StateStatusServerBoundHandshake
}

func (pk ServerBoundHandshake) IsLoginRequest() bool {
	return pk.NextState == StateLoginServerBoundHandshake
}

func (pk ServerBoundHandshake) IsForgeAddress() bool {
	addr := string(pk.ServerAddress)
	return len(strings.Split(addr, SeparatorForge)) > 1
}

func (pk ServerBoundHandshake) IsRealIPAddress() bool {
	addr := string(pk.ServerAddress)
	return len(strings.Split(addr, SeparatorRealIP)) > 1
}

func (pk ServerBoundHandshake) ParseServerAddress() string {
	addr := string(pk.ServerAddress)
	addr = strings.Split(addr, SeparatorForge)[0]
	addr = strings.Split(addr, SeparatorRealIP)[0]
	// Resolves an issue with some proxies
	addr = strings.Trim(addr, ".")
	return addr
}

func parseTCPAddr(addr string) (net.Addr, error) {
	ipStr, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	return &net.TCPAddr{
		IP:   net.ParseIP(ipStr),
		Port: port,
	}, nil
}

func (pk ServerBoundHandshake) ParseRealIP() (net.Addr, time.Time, []byte, error) {
	payload := strings.Split(string(pk.ServerAddress), SeparatorRealIP)
	if len(payload) < 4 {
		return nil, time.Time{}, nil, errors.New("invalid payload")
	}

	addr, err := parseTCPAddr(payload[1])
	if err != nil {
		return nil, time.Time{}, nil, err
	}

	timeStamp, err := time.Parse(time.UnixDate, payload[2])
	if err != nil {
		return nil, time.Time{}, nil, err
	}

	return addr, timeStamp, []byte(payload[3]), nil
}

func (pk *ServerBoundHandshake) UpgradeToRealIP(clientAddr net.Addr, timestamp time.Time) {
	addr := string(pk.ServerAddress)
	addrWithForge := strings.SplitN(addr, SeparatorForge, 3)

	addr = fmt.Sprintf("%s///%s///%d", addrWithForge[0], clientAddr.String(), timestamp.Unix())

	if len(addrWithForge) > 1 {
		addr = fmt.Sprintf("%s\x00%s\x00", addr, addrWithForge[1])
	}

	pk.ServerAddress = protocol.String(addr)
}

package handshaking

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
)

const (
	ServerBoundHandshakeID int32 = 0x00

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

func (pk ServerBoundHandshake) Marshal(packet *protocol.Packet) error {
	return packet.Encode(
		ServerBoundHandshakeID,
		pk.ProtocolVersion,
		pk.ServerAddress,
		pk.ServerPort,
		pk.NextState,
	)
}

func (pk *ServerBoundHandshake) Unmarshal(packet protocol.Packet) error {
	if packet.ID != ServerBoundHandshakeID {
		return protocol.ErrInvalidPacketID
	}

	return packet.Decode(
		&pk.ProtocolVersion,
		&pk.ServerAddress,
		&pk.ServerPort,
		&pk.NextState,
	)
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
	if i := strings.Index(addr, SeparatorForge); i != -1 {
		addr = addr[:i]
	}
	if i := strings.Index(addr, SeparatorRealIP); i != -1 {
		addr = addr[:i]
	}
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

	if len(addrWithForge) > 0 {
		addr = fmt.Sprintf("%s///%s///%d", addrWithForge[0], clientAddr.String(), timestamp.Unix())
	}

	if len(addrWithForge) > 1 {
		addr = fmt.Sprintf("%s\x00%s\x00", addr, addrWithForge[1])
	}

	pk.ServerAddress = protocol.String(addr)
}

package bedrock

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"strings"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/login"
	"github.com/pires/go-proxyproto"
)

// Processing Node
type ConnProcessor struct{}

func (cp ConnProcessor) ProcessConn(c net.Conn) (infrared.ProcessedConn, error) {
	pc := ProcessedConn{
		Conn:       c.(*Conn),
		remoteAddr: c.RemoteAddr(),
	}

	if pc.proxyProtocol {
		header, err := proxyproto.Read(bufio.NewReader(c))
		if err != nil {
			return nil, err
		}
		pc.remoteAddr = header.SourceAddr
	}

	b, err := pc.ReadPacket()
	if err != nil {
		return nil, err
	}
	pc.readBytes = b

	decoder := protocol.NewDecoder(bytes.NewReader(b))
	pks, err := decoder.Decode()
	if err != nil {
		return nil, err
	}

	if len(pks) < 1 {
		return nil, errors.New("no valid packets received")
	}

	var loginPk protocol.Login
	if err := protocol.UnmarshalPacket(pks[0], &loginPk); err != nil {
		return nil, err
	}

	iData, cData, err := login.Parse(loginPk.ConnectionRequest)
	if err != nil {
		return nil, err
	}
	pc.username = iData.DisplayName
	pc.serverAddr = cData.ServerAddress

	if strings.Contains(pc.serverAddr, ":") {
		pc.serverAddr, _, err = net.SplitHostPort(pc.serverAddr)
		if err != nil {
			return nil, err
		}
	}

	return &pc, nil
}

package java

import (
	"encoding/json"
	"fmt"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
)

type PlayerDisconnecter struct {
	statusJSON string
	message    string
}

func NewPlayerDisconnecter(responseJSON status.ResponseJSON, message string) (*PlayerDisconnecter, error) {
	bb, err := json.Marshal(responseJSON)
	if err != nil {
		return nil, err
	}

	return &PlayerDisconnecter{
		statusJSON: string(bb),
		message:    message,
	}, nil
}

func (d PlayerDisconnecter) DisconnectPlayer(p infrared.Player, opts ...infrared.MessageOption) error {
	defer p.Close()
	player := p.(*Player)

	msg := d.message
	for _, opt := range opts {
		msg = opt(msg)
	}

	if p.IsLoginRequest() {
		pk := login.ClientBoundDisconnect{
			Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
		}.Marshal()
		return player.WritePacket(pk)
	}

	pk := status.ClientBoundResponse{
		JSONResponse: protocol.String(msg),
	}.Marshal()

	if err := player.WritePacket(pk); err != nil {
		return err
	}

	ping, err := player.ReadPacket(status.MaxSizeServerBoundPingRequest)
	if err != nil {
		return err
	}

	return player.WritePacket(ping)
}

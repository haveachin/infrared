package bedrock

import (
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock/protocol/packet"
)

type PlayerDisconnecter struct {
	message string
}

func NewPlayerDisconnecter(message string) *PlayerDisconnecter {
	return &PlayerDisconnecter{
		message: message,
	}
}

func (d PlayerDisconnecter) DisconnectPlayer(p infrared.Player, opts ...infrared.MessageOption) error {
	defer p.Close()
	player := p.(*Player)

	msg := d.message
	for _, opt := range opts {
		msg = opt(msg)
	}

	pk := packet.Disconnect{
		HideDisconnectionScreen: msg == "",
		Message:                 msg,
	}

	if err := player.WritePacket(&pk); err != nil {
		return err
	}

	return nil
}

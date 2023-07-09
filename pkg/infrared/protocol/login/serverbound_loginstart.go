package login

import (
	"bytes"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
)

const (
	// MaxSizeServerBoundLoginStart might be a bit generous, but there is no offical max size for the public key
	MaxSizeServerBoundLoginStart       = 1 + 16*4 + 1 + 8 + 3000 + 3000 + 1 + 16
	IDServerBoundLoginStart      int32 = 0x00
)

type ServerBoundLoginStart struct {
	Name         protocol.String
	HasPublicKey protocol.Boolean

	// Added in 1.19; removed in 1.19.3
	Timestamp protocol.Long
	PublicKey protocol.ByteArray
	Signature protocol.ByteArray

	// Added in 1.19
	HasPlayerUUID protocol.Boolean
	PlayerUUID    protocol.UUID
}

func (pk ServerBoundLoginStart) Marshal(packet *protocol.Packet, version protocol.Version) {
	if version < protocol.Version_1_19 {
		packet.Encode(
			IDServerBoundLoginStart,
			pk.Name,
		)
		return
	}

	fields := make([]protocol.FieldEncoder, 0, 7)
	fields = append(fields, pk.Name, pk.HasPublicKey)
	if pk.HasPublicKey {
		fields = append(fields, pk.Timestamp, pk.PublicKey, pk.Signature)
	}
	fields = append(fields, pk.HasPlayerUUID)
	if pk.HasPlayerUUID {
		fields = append(fields, pk.PlayerUUID)
	}

	packet.Encode(
		IDServerBoundLoginStart,
		fields...,
	)
}

func (pk *ServerBoundLoginStart) Unmarshal(packet protocol.Packet, version protocol.Version) error {
	if packet.ID != IDServerBoundLoginStart {
		return protocol.ErrInvalidPacketID
	}

	r := bytes.NewReader(packet.Data)
	if err := protocol.ScanFields(r, &pk.Name); err != nil {
		return err
	}

	if version < protocol.Version_1_19 {
		return nil
	}

	if version < protocol.Version_1_19_3 {
		if err := protocol.ScanFields(r, &pk.HasPublicKey); err != nil {
			return err
		}

		if pk.HasPublicKey {
			if err := protocol.ScanFields(r, &pk.Timestamp, &pk.PublicKey, &pk.Signature); err != nil {
				return err
			}
		}
	}

	if err := protocol.ScanFields(r, &pk.HasPlayerUUID); err != nil {
		return err
	}

	if pk.HasPlayerUUID {
		if err := protocol.ScanFields(r, &pk.PlayerUUID); err != nil {
			return err
		}
	}

	return nil
}

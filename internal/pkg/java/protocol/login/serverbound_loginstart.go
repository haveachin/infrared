package login

import (
	"bytes"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
)

const (
	// MaxSizeServerBoundLoginStart might be a bit generous, but there is no offical max size for the public key
	MaxSizeServerBoundLoginStart      = 1 + 16 + 1 + 8 + 3000 + 3000 + 1 + 16
	IDServerBoundLoginStart      byte = 0x00
)

type ServerLoginStart struct {
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

func (pk ServerLoginStart) Marshal(version int32) protocol.Packet {
	if version < protocol.Version_1_19 {
		return protocol.MarshalPacket(
			IDServerBoundLoginStart,
			pk.Name,
		)
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

	return protocol.MarshalPacket(
		IDServerBoundLoginStart,
		fields...,
	)
}

func UnmarshalServerBoundLoginStart(packet protocol.Packet, version int32) (ServerLoginStart, error) {
	var pk ServerLoginStart

	if packet.ID != IDServerBoundLoginStart {
		return pk, protocol.ErrInvalidPacketID
	}

	r := bytes.NewReader(packet.Data)
	if err := protocol.ScanFields(r, &pk.Name); err != nil {
		return pk, err
	}

	if version < protocol.Version_1_19 {
		return pk, nil
	}

	if version < protocol.Version_1_19_3 {
		if err := protocol.ScanFields(r, &pk.HasPublicKey); err != nil {
			return pk, err
		}

		if pk.HasPublicKey {
			if err := protocol.ScanFields(r, &pk.Timestamp, &pk.PublicKey, &pk.Signature); err != nil {
				return pk, err
			}
		}
	}

	if err := protocol.ScanFields(r, &pk.HasPlayerUUID); err != nil {
		return pk, err
	}

	if pk.HasPlayerUUID {
		if err := protocol.ScanFields(r, &pk.PlayerUUID); err != nil {
			return pk, err
		}
	}

	return pk, nil
}

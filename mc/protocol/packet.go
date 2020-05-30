package protocol

import (
	pk "github.com/haveachin/infrared/mc/packet"
	"reflect"
)

type Packet interface {
	ID() byte
}

func Parse(dataPacket pk.Packet, structPacket Packet) error {
	if dataPacket.ID != structPacket.ID() {
		return ErrInvalidPacketID
	}

	vStructPacket := reflect.ValueOf(structPacket)

	if vStructPacket.Kind() != reflect.Ptr {
		return ErrNotAPointer
	}

	vStructPacket = vStructPacket.Elem()
	fieldAddr := make([]pk.FieldDecoder, vStructPacket.NumField())

	for i := 0; i < vStructPacket.NumField(); i++ {
		field := vStructPacket.Field(i)
		fieldAddr[i] = field.Addr().Interface().(pk.FieldDecoder)
	}

	if err := dataPacket.Scan(fieldAddr...); err != nil {
		return err
	}

	return nil
}

package speed_test

import (
	"bytes"
	"github.com/haveachin/infrared/protocol/speed"
	"testing"

	"github.com/haveachin/infrared/protocol/handshaking"
)


func BenchmarkHandshakingVersionOld_WithoutData(b *testing.B) {
	pk := handshaking.ServerBoundHandshake{
		
	}.Marshal()
	data, _ := pk.Marshal()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(data)

		speed.DomainOldWay(buf)
	}
}

func BenchmarkHandshakeVersionOld_WithData(b *testing.B) {
	pk := handshaking.ServerBoundHandshake{
		ProtocolVersion: 751,
		NextState: 1,
		ServerAddress: "play.infrared.nl",
		ServerPort: 25565,
	}.Marshal()
	data, _ := pk.Marshal()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(data)
		speed.DomainOldWay(buf)
	}
}

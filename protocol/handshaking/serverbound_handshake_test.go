package handshaking

import "testing"

func BenchmarkHandshakingServerBoundHandshake_Marshal(b *testing.B) {
	isHandshakePk := ServerBoundHandshake{
		ProtocolVersion: 578,
		ServerAddress:   "spook.space",
		ServerPort:      25565,
		NextState:       1,
	}

	pk := isHandshakePk.Marshal()

	for n := 0; n < b.N; n++ {
		if _, err := UnmarshalServerBoundHandshake(pk); err != nil {
			b.Error(err)
		}
	}
}

package protocol

import "strconv"

type Version int32

const (
	Version1_18_2 Version = 758
	Version1_19   Version = 759
	Version1_19_3 Version = 761
	Version1_20_2 Version = 764
)

func (v Version) Name() string {
	switch v {
	case Version1_18_2:
		return "1.18.2"
	case Version1_19:
		return "1.19"
	case Version1_19_3:
		return "1.19.3"
	case Version1_20_2:
		return "1.20.2"
	default:
		return strconv.Itoa(int(v))
	}
}

func (v Version) ProtocolNumber() int32 {
	return int32(v)
}

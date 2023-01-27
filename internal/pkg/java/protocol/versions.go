package protocol

import "strconv"

type Version int32

const (
	Version_1_18_2 Version = 758
	Version_1_19   Version = 759
	Version_1_19_3 Version = 761
)

func (v Version) Name() string {
	switch v {
	case Version_1_18_2:
		return "1.18.2"
	case Version_1_19:
		return "1.19"
	case Version_1_19_3:
		return "1.19.3"
	default:
		return strconv.Itoa(int(v))
	}
}

func (v Version) ProtocolNumber() int32 {
	return int32(v)
}

package protocol

type State byte

const (
	StateHandshaking State = iota
	StateStatus
	StateLogin
	StatePlay
)

func (state State) String() string {
	names := map[State]string{
		StateHandshaking: "Handshaking",
		StateStatus:      "Status",
		StateLogin:       "Login",
		StatePlay:        "Play",
	}

	return names[state]
}

func (state State) IsHandshaking() bool {
	return state == StateHandshaking
}

func (state State) IsStatus() bool {
	return state == StateStatus
}

func (state State) IsLogin() bool {
	return state == StateLogin
}

func (state State) IsPlay() bool {
	return state == StatePlay
}

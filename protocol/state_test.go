package protocol

import (
	"testing"
)

func TestState_String(t *testing.T) {
	tt := []struct {
		state State
		str   string
	}{
		{
			state: StateHandshaking,
			str:   "Handshaking",
		},
		{
			state: StateStatus,
			str:   "Status",
		},
		{
			state: StateLogin,
			str:   "Login",
		},
		{
			state: StatePlay,
			str:   "Play",
		},
	}

	for _, tc := range tt {
		if tc.state.String() != tc.str {
			t.Errorf("got: %v; want: %v", tc.state.String(), tc.str)
		}
	}
}

func TestState_IsHandshaking(t *testing.T) {
	tt := []struct {
		state  State
		result bool
	}{
		{
			state:  StateHandshaking,
			result: true,
		},
		{
			state:  StateStatus,
			result: false,
		},
	}

	for _, tc := range tt {
		if tc.state.IsHandshaking() != tc.result {
			t.Fail()
		}
	}
}

func TestState_IsStatus(t *testing.T) {
	tt := []struct {
		state  State
		result bool
	}{
		{
			state:  StateStatus,
			result: true,
		},
		{
			state:  StatePlay,
			result: false,
		},
	}

	for _, tc := range tt {
		if tc.state.IsStatus() != tc.result {
			t.Fail()
		}
	}
}

func TestState_IsLogin(t *testing.T) {
	tt := []struct {
		state  State
		result bool
	}{
		{
			state:  StateLogin,
			result: true,
		},
		{
			state:  StatePlay,
			result: false,
		},
	}

	for _, tc := range tt {
		if tc.state.IsLogin() != tc.result {
			t.Fail()
		}
	}
}

func TestState_IsPlay(t *testing.T) {
	tt := []struct {
		state  State
		result bool
	}{
		{
			state:  StatePlay,
			result: true,
		},
		{
			state:  StateLogin,
			result: false,
		},
	}

	for _, tc := range tt {
		if tc.state.IsPlay() != tc.result {
			t.Fail()
		}
	}
}

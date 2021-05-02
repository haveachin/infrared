package infrared

import (
	"testing"
)

func TestStatusResponsePacket(t *testing.T) {
	samples := make([]PlayerSample, 0)
	config := StatusConfig{
		VersionName:    "Server\"{TEST}\"",
		ProtocolNumber: 754,
		MaxPlayers:     20,
		PlayersOnline:  0,
		PlayerSamples:  samples,
		MOTD:           "Server MOTD",
	}

	pk, err := config.StatusResponsePacket()
	if err != nil {
		t.Errorf("Error in converting config to packet: %v", err)
	}

	response, err := statusReponseToStruct(pk)
	if err != nil {
		t.Errorf("Error in converting packet to response struct: %v", err)
	}

	if config.VersionName != response.Version.Name {
		t.Errorf("Different version name - expected: %v, got: %v", config.VersionName, response.Version.Name)
	}
	if config.ProtocolNumber != response.Version.Protocol {
		t.Errorf("Different protocol versions - expected: %v, got: %v", config.VersionName, response.Version.Protocol)
	}
	if config.MaxPlayers != response.Players.Max {
		t.Errorf("Different max players - expected: %v, got: %v", config.MaxPlayers, response.Players.Max)
	}
	if config.PlayersOnline != response.Players.Online {
		t.Errorf("Different amount of online players - expected: %v, got: %v", config.PlayersOnline, response.Players.Online)
	}
	if config.MOTD != response.Description.Text {
		t.Errorf("Different descriptions - expected: %v, got: %v", config.MOTD, response.Description.Text)
	}
	// Needs more indept comparison and matching types
	// if config.PlayerSamples != response.Players.Sample {
	// 	t.Errorf("Different max players - expected: %v, got: %v", config.PlayerSamples, response.Players.Sample)
	// }

	// Needs to be worked on
	// sameImage := response.Favicon == config.IconPath

}

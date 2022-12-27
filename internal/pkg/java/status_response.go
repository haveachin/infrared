package java

import (
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
)

type PlayerSample struct {
	Name string
	UUID string
}

type PlayerSamples []PlayerSample

func (ps PlayerSamples) PlayerSampleJSON() []status.PlayerSampleJSON {
	ss := make([]status.PlayerSampleJSON, len(ps))
	for i, s := range ps {
		ss[i] = status.PlayerSampleJSON{
			Name: s.Name,
			ID:   s.UUID,
		}
	}
	return ss
}

type OverrideStatusResponse struct {
	VersionName    *string
	ProtocolNumber *int
	MaxPlayerCount *int
	PlayerCount    *int
	PlayerSamples  PlayerSamples
	Icon           *string
	MOTD           *string
}

func (r OverrideStatusResponse) ResponseJSON(resp status.ResponseJSON, pc *ProcessedConn, s Server) status.ResponseJSON {
	if r.Icon != nil {
		resp.Favicon = *r.Icon
	}

	if r.VersionName != nil {
		resp.Version.Name = *r.VersionName
	}

	if r.ProtocolNumber != nil {
		resp.Version.Protocol = *r.ProtocolNumber
	}

	if r.MaxPlayerCount != nil {
		resp.Players.Max = *r.MaxPlayerCount
	}

	if r.PlayerCount != nil {
		resp.Players.Online = *r.PlayerCount
	}

	if r.PlayerSamples != nil {
		resp.Players.Sample = r.PlayerSamples.PlayerSampleJSON()
	}

	if r.MOTD != nil {
		resp.Description = status.DescriptionJSON{
			Text: infrared.ExecuteServerMessageTemplate(*r.MOTD, pc, s.InfraredServer()),
		}
	}

	return resp
}

func (r OverrideStatusResponse) ExecuteServerMessageTemplate(pc ProcessedConn, s Server) {

}

type ServerStatusResponse struct {
	VersionName    string
	ProtocolNumber int
	MaxPlayerCount int
	PlayerCount    int
	PlayerSamples  PlayerSamples
	Icon           string
	MOTD           string
}

func (r ServerStatusResponse) ResponseJSON() status.ResponseJSON {
	return status.ResponseJSON{
		Version: status.VersionJSON{
			Name:     r.VersionName,
			Protocol: r.ProtocolNumber,
		},
		Players: status.PlayersJSON{
			Max:    r.MaxPlayerCount,
			Online: r.PlayerCount,
			Sample: r.PlayerSamples.PlayerSampleJSON(),
		},
		Favicon: r.Icon,
		Description: status.DescriptionJSON{
			Text: r.MOTD,
		},
	}
}

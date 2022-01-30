package java

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

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
	IconPath       *string
	MOTD           *string
}

func (r OverrideStatusResponse) ResponseJSON(resp status.ResponseJSON) (status.ResponseJSON, error) {
	if r.IconPath != nil {
		var err error
		resp.Favicon, err = loadImageAndEncodeToBase64String(*r.IconPath)
		if err != nil {
			return status.ResponseJSON{}, err
		}
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
		resp.Description = status.DescriptionJSON{Text: *r.MOTD}
	}

	return resp, nil
}

type DialTimeoutStatusResponse struct {
	VersionName    string
	ProtocolNumber int
	MaxPlayerCount int
	PlayerCount    int
	PlayerSamples  PlayerSamples
	IconPath       string
	MOTD           string
}

func (r DialTimeoutStatusResponse) ResponseJSON() (status.ResponseJSON, error) {
	img64, err := loadImageAndEncodeToBase64String(r.IconPath)
	if err != nil {
		return status.ResponseJSON{}, err
	}

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
		Favicon: img64,
		Description: status.DescriptionJSON{
			Text: r.MOTD,
		},
	}, nil
}

func loadImageAndEncodeToBase64String(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	imgFile, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer imgFile.Close()

	bb, err := io.ReadAll(imgFile)
	if err != nil {
		return "", err
	}
	img64 := base64.StdEncoding.EncodeToString(bb)

	return fmt.Sprintf("data:image/png;base64,%s", img64), nil
}

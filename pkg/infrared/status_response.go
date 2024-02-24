package infrared

import (
	"encoding/base64"
	_ "image/png"
	"os"
	"strings"
	"sync"

	"github.com/haveachin/infrared/pkg/infrared/protocol/status"
)

type PlayerSampleConfig struct {
	Name string `yaml:"name"`
	UUID string `yaml:"uuid"`
}

type PlayerSamples []PlayerSampleConfig

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

type ServerStatusResponseConfig struct {
	VersionName    string        `yaml:"versionName"`
	ProtocolNumber int           `yaml:"protocolNumber"`
	MaxPlayerCount int           `yaml:"maxPlayerCount"`
	PlayerCount    int           `yaml:"playerCount"`
	PlayerSamples  PlayerSamples `yaml:"playerSamples"`
	Icon           string        `yaml:"icon"`
	MOTD           string        `yaml:"motd"`

	once     sync.Once
	respJSON status.ResponseJSON
}

func (r ServerStatusResponseConfig) ResponseJSON() status.ResponseJSON {
	r.once.Do(func() {
		r.respJSON = status.ResponseJSON{
			Version: status.VersionJSON{
				Name:     r.VersionName,
				Protocol: r.ProtocolNumber,
			},
			Players: status.PlayersJSON{
				Max:    r.MaxPlayerCount,
				Online: r.PlayerCount,
				Sample: r.PlayerSamples.PlayerSampleJSON(),
			},
			Favicon: parseServerIcon(r.Icon),
			Description: status.DescriptionJSON{
				Text: r.MOTD,
			},
		}
	})

	return r.respJSON
}

type OverrideServerStatusResponseConfig struct {
	VersionName    *string       `yaml:"versionName"`
	ProtocolNumber *int          `yaml:"protocolNumber"`
	MaxPlayerCount *int          `yaml:"maxPlayerCount"`
	PlayerCount    *int          `yaml:"playerCount"`
	PlayerSamples  PlayerSamples `yaml:"playerSamples"`
	Icon           *string       `yaml:"icon"`
	MOTD           *string       `yaml:"motd"`

	iconOnce sync.Once
}

func (r OverrideServerStatusResponseConfig) OverrideResponseJSON(resp status.ResponseJSON) status.ResponseJSON {
	if r.Icon != nil {
		r.iconOnce.Do(func() {
			icon := parseServerIcon(*r.Icon)
			r.Icon = &icon
		})
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

	if len(r.PlayerSamples) != 0 {
		resp.Players.Sample = r.PlayerSamples.PlayerSampleJSON()
	}

	if r.MOTD != nil {
		resp.Description = status.DescriptionJSON{
			Text: *r.MOTD,
		}
	}

	return resp
}

func parseServerIcon(iconStr string) string {
	if iconStr == "" {
		return ""
	}

	const base64PNGHeader = "data:image/png;base64,"
	if strings.HasPrefix(iconStr, base64PNGHeader) {
		return iconStr
	}

	iconBytes, err := os.ReadFile(iconStr)
	if err != nil {
		Log.Error().
			Err(err).
			Str("iconPath", iconStr).
			Msg("Failed to open icon file")
		return ""
	}

	iconBase64 := base64.StdEncoding.EncodeToString(iconBytes)
	return base64PNGHeader + iconBase64
}

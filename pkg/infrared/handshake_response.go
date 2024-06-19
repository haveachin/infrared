package infrared

import (
	"encoding/base64"
	"encoding/json"
	_ "image/png"
	"os"
	"strings"
	"sync"

	"github.com/haveachin/infrared/pkg/infrared/protocol"
	"github.com/haveachin/infrared/pkg/infrared/protocol/login"
	"github.com/haveachin/infrared/pkg/infrared/protocol/status"
)

type PlayerSampleConfig struct {
	Name string `yaml:"name"`
	UUID string `yaml:"uuid"`
}

type PlayerSamples []PlayerSampleConfig

func (ps PlayerSamples) PlayerSampleJSON() []status.PlayerSampleJSON {
	psJSON := make([]status.PlayerSampleJSON, len(ps))
	for i, s := range ps {
		psJSON[i] = status.PlayerSampleJSON{
			Name: s.Name,
			ID:   s.UUID,
		}
	}
	return psJSON
}

type HandshakeStatusResponseConfig struct {
	VersionName    string        `yaml:"versionName"`
	ProtocolNumber int           `yaml:"protocolNumber"`
	MaxPlayerCount int           `yaml:"maxPlayerCount"`
	PlayerCount    int           `yaml:"playerCount"`
	PlayerSamples  PlayerSamples `yaml:"playerSamples"`
	Icon           string        `yaml:"icon"`
	MOTD           string        `yaml:"motd"`
}

func (cfg HandshakeStatusResponseConfig) IsProtocolNumberDynamic() bool {
	return cfg.ProtocolNumber == -1
}

type HandshakeResponseConfig struct {
	StatusConfig HandshakeStatusResponseConfig `yaml:"status"`
	Message      string                        `yaml:"message"`
}

type HandshakeResponse struct {
	Config HandshakeResponseConfig

	statusOnce   sync.Once
	statusResp   status.ClientBoundResponse
	statusRespPk protocol.Packet

	loginOnce   sync.Once
	loginRespPk protocol.Packet
}

func (r *HandshakeResponse) StatusResponse(protVer protocol.Version) protocol.Packet {
	cfg := r.Config.StatusConfig

	r.statusOnce.Do(func() {
		protVer = protocol.Version(cfg.ProtocolNumber)
		r.renderStatusPacket(protVer)
	})

	if cfg.IsProtocolNumberDynamic() {
		if err := r.statusResp.SetVersionProtocol(protVer); err != nil {
			panic(err)
		}

		if err := r.statusResp.Marshal(&r.statusRespPk); err != nil {
			panic(err)
		}
	}

	return r.statusRespPk
}

func (r *HandshakeResponse) renderStatusPacket(protVer protocol.Version) {
	cfg := r.Config.StatusConfig
	respJSON := status.ResponseJSON{
		Version: status.VersionJSON{
			Name:     cfg.VersionName,
			Protocol: int(protVer),
		},
		Players: status.PlayersJSON{
			Max:    cfg.MaxPlayerCount,
			Online: cfg.PlayerCount,
			Sample: cfg.PlayerSamples.PlayerSampleJSON(),
		},
		Favicon:     parseServerIcon(cfg.Icon),
		Description: parseJSONTextComponent(cfg.MOTD),
	}

	respBytes, err := json.Marshal(respJSON)
	if err != nil {
		panic(err)
	}

	r.statusResp = status.ClientBoundResponse{
		JSONResponse: protocol.String(string(respBytes)),
	}
}

func (r *HandshakeResponse) LoginReponse() protocol.Packet {
	r.loginOnce.Do(func() {
		msg := parseJSONTextComponent(r.Config.Message)
		disconnectPk := login.ClientBoundDisconnect{
			Reason: protocol.String(msg),
		}

		if err := disconnectPk.Marshal(&r.loginRespPk); err != nil {
			panic(err)
		}
	})

	return r.loginRespPk
}

type OverrideStatusResponseConfig struct {
	VersionName    *string       `yaml:"versionName"`
	ProtocolNumber *int          `yaml:"protocolNumber"`
	MaxPlayerCount *int          `yaml:"maxPlayerCount"`
	PlayerCount    *int          `yaml:"playerCount"`
	PlayerSamples  PlayerSamples `yaml:"playerSamples"`
	Icon           *string       `yaml:"icon"`
	MOTD           *string       `yaml:"motd"`
}

type OverrideStatusResponse struct {
	Config OverrideStatusResponseConfig

	once       sync.Once
	overrideFn func(resp status.ResponseJSON) status.ResponseJSON
}

func (r *OverrideStatusResponse) OverrideStatusResponseJSON(resp status.ResponseJSON) status.ResponseJSON {
	r.once.Do(func() {
		cfg := r.Config
		icon := parseServerIcon(*cfg.Icon)
		playerSamples := cfg.PlayerSamples.PlayerSampleJSON()
		motd := parseJSONTextComponent(*cfg.MOTD)

		r.overrideFn = func(resp status.ResponseJSON) status.ResponseJSON {
			if cfg.Icon != nil {
				resp.Favicon = icon
			}

			if cfg.VersionName != nil {
				resp.Version.Name = *cfg.VersionName
			}

			if cfg.ProtocolNumber != nil {
				resp.Version.Protocol = *cfg.ProtocolNumber
			}

			if cfg.MaxPlayerCount != nil {
				resp.Players.Max = *cfg.MaxPlayerCount
			}

			if cfg.PlayerCount != nil {
				resp.Players.Online = *cfg.PlayerCount
			}

			if len(cfg.PlayerSamples) != 0 {
				resp.Players.Sample = playerSamples
			}

			if cfg.MOTD != nil {
				resp.Description = motd
			}

			return resp
		}
	})

	return r.overrideFn(resp)
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

func parseJSONTextComponent(s string) json.RawMessage {
	var motdJSON json.RawMessage
	if err := json.Unmarshal([]byte(s), &motdJSON); err != nil {
		motdJSON = []byte(`{"text":"` + s + `"}`)
	}
	return motdJSON
}

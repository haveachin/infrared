package sim

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

type ServerConfig struct {
	DisconnectMessage string
	Version           string
	Protocol          int
	Icon              string
	Motd              string
	MaxPlayers        int
	PlayersOnline     int
	Players           []struct {
		Name string
		ID   string
	}
}

func (cfg ServerConfig) marshalPingResponse() ([]byte, error) {
	response := struct {
		Version struct {
			Name     string `json:"name"`
			Protocol int    `json:"protocol"`
		} `json:"version"`
		Players struct {
			Max    int `json:"max"`
			Online int `json:"online"`
			Sample []struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			} `json:"sample"`
		} `json:"players"`
		Description struct {
			Text string `json:"text"`
		} `json:"description"`
		Favicon string `json:"favicon"`
	}{}

	response.Version.Name = cfg.Version
	response.Version.Protocol = cfg.Protocol
	response.Players.Max = cfg.MaxPlayers
	response.Players.Online = cfg.PlayersOnline
	response.Description.Text = cfg.Motd

	for _, p := range cfg.Players {
		response.Players.Sample = append(response.Players.Sample,
			struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			}{
				Name: p.Name,
				ID:   p.ID,
			},
		)
	}

	if cfg.Icon != "" {
		img64, err := loadImageToBase64String(cfg.Icon)
		if err != nil {
			return nil, err
		}

		response.Favicon = fmt.Sprintf("data:image/png;base64,%s", img64)
	}

	return json.Marshal(response)
}

func loadImageToBase64String(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	imgFile, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer imgFile.Close()

	fileInfo, err := imgFile.Stat()
	if err != nil {
		return "", err
	}

	buffer := make([]byte, fileInfo.Size())
	fileReader := bufio.NewReader(imgFile)
	_, err = fileReader.Read(buffer)
	if err != nil {
		return "", nil
	}

	return base64.StdEncoding.EncodeToString(buffer), nil
}

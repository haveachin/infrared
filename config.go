package infrared

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DomainName        string
	ListenTo          string
	ProxyTo           string
	Deadline          string
	PingCommand       string
	DisconnectMessage string
	Placeholder       struct {
		Version       string
		Protocol      int
		Icon          string
		Motd          string
		MaxPlayers    int
		PlayersOnline int
		Players       []struct {
			Name string
			ID   string
		}
	}
	PlaceholderJSON string
}

func ReadConfigs(path string) ([]Config, error) {
	viper.AddConfigPath(path)
	viper.SetConfigType("yaml")

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	configs := []Config{}

	for _, file := range files {
		log.Println("Loading", file.Name())
		viper.SetConfigName(strings.TrimSuffix(file.Name(), ".yaml"))

		if err := viper.ReadInConfig(); err != nil {
			return nil, err
		}

		var config Config
		if err := viper.Unmarshal(&config); err != nil {
			return nil, err
		}

		placeholderJSON, err := config.marshalPlaceholder()
		if err != nil {
			return nil, err
		}

		config.PlaceholderJSON = string(placeholderJSON)
		configs = append(configs, config)
	}

	return configs, nil
}

func (config Config) marshalPlaceholder() ([]byte, error) {
	placeholder := placeholder{
		Version: version{
			Name:     config.Placeholder.Version,
			Protocol: config.Placeholder.Protocol,
		},
		Description: description{
			Text: config.Placeholder.Motd,
		},
		Players: players{
			Max:    config.Placeholder.MaxPlayers,
			Online: config.Placeholder.PlayersOnline,
			Sample: []player{},
		},
	}

	for _, p := range config.Placeholder.Players {
		placeholder.Players.Sample = append(placeholder.Players.Sample, player{
			Name: p.Name,
			ID:   p.ID,
		})
	}

	img64, err := loadImageToBase64String(config.Placeholder.Icon)
	if err != nil {
		return nil, err
	}

	placeholder.Favicon = fmt.Sprintf("data:image/png;base64,%s", img64)

	return json.Marshal(placeholder)
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

	fInfo, err := imgFile.Stat()
	if err != nil {
		return "", err
	}

	buffer := make([]byte, fInfo.Size())
	fReader := bufio.NewReader(imgFile)
	fReader.Read(buffer)

	return base64.StdEncoding.EncodeToString(buffer), nil
}

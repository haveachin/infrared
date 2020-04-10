package infrared

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Config is a data representation of a proxy configuration
type Config struct {
	DomainName        string
	ListenTo          string
	ProxyTo           string
	DNSAddress        string
	DisconnectMessage string
	Timeout           string
	Docker            struct {
		ContainerName string
		CallbackURL   string
		Portainer     struct {
			Address    string
			EndpointID string
			Username   string
			Password   string
		}
	}
	Placeholder struct {
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
}

// ReadAllConfigs reads all files that are in the given path
func ReadAllConfigs(path string) ([]*viper.Viper, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	vprs := []*viper.Viper{}

	for _, file := range files {
		fileName := file.Name()

		log.Info().Msgf("Loading \"%s\"", fileName)

		extension := filepath.Ext(fileName)
		configName := fileName[0 : len(fileName)-len(extension)]

		vpr := viper.New()
		vpr.AddConfigPath(path)
		vpr.SetConfigName(configName)
		vpr.SetConfigType(strings.TrimPrefix(extension, "."))
		loadDefaults(vpr)

		vprs = append(vprs, vpr)
	}

	return vprs, nil
}

// LoadConfig loads the config from the viper configuration
func LoadConfig(vpr *viper.Viper) (Config, error) {
	config := Config{}

	if err := vpr.ReadInConfig(); err != nil {
		return config, err
	}

	if err := vpr.Unmarshal(&config); err != nil {
		return config, err
	}

	return config, nil
}

// UsesDocker returns a bool that determines if the config has data to support a docker process
func (config Config) UsesDocker() bool {
	return config.Docker.ContainerName != ""
}

// UsesPortainer returns a bool that determines if the config has data to support a portainer process
func (config Config) UsesPortainer() bool {
	if !config.UsesDocker() {
		return false
	}

	if config.Docker.Portainer.EndpointID == "" {
		return false
	}

	if config.Docker.Portainer.Username == "" {
		return false
	}

	if config.Docker.Portainer.Password == "" {
		return false
	}

	return true
}

// MarshalPlaceholder marshals the placeholder data to a SLP placeholder response object
func (config Config) MarshalPlaceholder() ([]byte, error) {
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
			Sample: []sample{},
		},
	}

	for _, p := range config.Placeholder.Players {
		placeholder.Players.Sample = append(placeholder.Players.Sample, sample{
			Name: p.Name,
			ID:   p.ID,
		})
	}

	if config.Placeholder.Icon != "" {
		img64, err := loadImageToBase64String(config.Placeholder.Icon)
		if err != nil {
			return nil, err
		}

		placeholder.Favicon = fmt.Sprintf("data:image/png;base64,%s", img64)
	}

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

func loadDefaults(vpr *viper.Viper) {
	vpr.SetDefault("DisconnectMessage", "Hey §e$username§r! The server was sleeping but it's starting now.")
	vpr.SetDefault("Timeout", "10m")
	vpr.SetDefault("Placeholder.Version", "Infrared 1.15.1")
	vpr.SetDefault("Placeholder.Protocol", 575)
	vpr.SetDefault("Placeholder.Motd", "Server is currently sleeping")
	vpr.SetDefault("Placeholder.MaxPlayers", 20)
	vpr.SetDefault("Placeholder.PlayersOnline", 0)
}

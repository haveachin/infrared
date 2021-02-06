package infrared

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	"github.com/haveachin/infrared/callback"
	"github.com/haveachin/infrared/process"
	"github.com/specspace/plasma"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrFileIsDir = errors.New("file is a dir")

// ProxyConfig is a data representation of a Proxy configuration
type ProxyConfig struct {
	sync.RWMutex
	vpr *viper.Viper

	DomainName        string
	ListenTo          string
	ProxyTo           string
	ProxyProtocol     bool
	Timeout           time.Duration
	DisconnectMessage string
	Docker            process.Config
	OnlineStatus      plasma.StatusResponse
	OfflineStatus     plasma.StatusResponse
	CallbackServer    callback.Config
}

func (cfg *ProxyConfig) setDefaults() {
	cfg.vpr.SetDefault("DomainName", "localhost")
	cfg.vpr.SetDefault("ListenTo", ":25565")
	cfg.vpr.SetDefault("ProxyProtocol", false)
	cfg.vpr.SetDefault("Timeout", "1s")
	cfg.vpr.SetDefault("DisconnectMessage", "Sorry {{username}}, but the server is offline.")

	cfg.vpr.SetDefault("Docker.DNSServer", "127.0.0.11")
	cfg.vpr.SetDefault("Docker.Timeout", "5m")

	cfg.vpr.SetDefault("OfflineStatus.DisconnectMessage", "Hey §e$username§r! The listeners was sleeping but it is starting now.")
	cfg.vpr.SetDefault("OfflineStatus.Version.Name", "Infrared 1.16.5")
	cfg.vpr.SetDefault("OfflineStatus.Version.ProtocolNumber", 754)
	cfg.vpr.SetDefault("OfflineStatus.PlayersInfo.MaxPlayers", 20)
	cfg.vpr.SetDefault("OfflineStatus.PlayersInfo.PlayersOnline", 0)
	cfg.vpr.SetDefault("OfflineStatus.MOTD", "Powered by Infrared")
}

func ReadFilePaths(path string, recursive bool) ([]string, error) {
	if recursive {
		return readFilePathsRecursively(path)
	}

	return readFilePaths(path)
}

func readFilePathsRecursively(path string) ([]string, error) {
	var filePaths []string

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		filePaths = append(filePaths, path)
		return nil
	})

	return filePaths, err
}

func readFilePaths(path string) ([]string, error) {
	var filePaths []string
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePaths = append(filePaths, filepath.Join(path, file.Name()))
	}

	return filePaths, err
}

func LoadProxyConfigsFromPath(path string, recursive bool) ([]*ProxyConfig, error) {
	filePaths, err := ReadFilePaths(path, recursive)
	if err != nil {
		return nil, err
	}

	var cfgs []*ProxyConfig

	for _, filePath := range filePaths {
		cfg, err := NewProxyConfigFromPath(filePath)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, cfg)
	}

	return cfgs, nil
}

// NewProxyConfigFromPath loads a ProxyConfig from a file path and then starts watching
// it for changes. On change the ProxyConfig will automatically Reload itself
func NewProxyConfigFromPath(path string) (*ProxyConfig, error) {
	log.Println("Loading", path)
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, ErrFileIsDir
	}

	fileName := stat.Name()
	extension := filepath.Ext(fileName)
	configName := fileName[:len(fileName)-len(extension)]

	vpr := viper.New()
	vpr.AddConfigPath(filepath.Dir(path))
	vpr.SetConfigName(configName)
	vpr.SetConfigType(extension[1:]) // [1:] removes the "."

	cfg := ProxyConfig{vpr: vpr}
	cfg.setDefaults()

	vpr.OnConfigChange(cfg.onConfigChange(path))
	if err := cfg.Reload(); err != nil {
		return nil, err
	}
	vpr.WatchConfig()

	return &cfg, err
}

func (cfg *ProxyConfig) onConfigChange(path string) func(fsnotify.Event) {
	return func(in fsnotify.Event) {
		if in.Op != fsnotify.Write {
			return
		}

		if err := cfg.Reload(); err != nil {
			log.Printf("Failed update on %s; error %s", path, err)
			return
		}

		log.Println("Updated", path)
	}
}

// Reload loads the ProxyConfig from it's source file
func (cfg *ProxyConfig) Reload() error {
	cfg.Lock()
	defer cfg.Unlock()

	if err := cfg.vpr.ReadInConfig(); err != nil {
		return err
	}

	if err := cfg.vpr.Unmarshal(cfg); err != nil {
		return err
	}

	// TODO: Trigger ProxyConfig changed event
	return nil
}

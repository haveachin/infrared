package infrared

import (
	"encoding/json"
	"github.com/fsnotify/fsnotify"
	"github.com/haveachin/infrared/process"
	"github.com/specspace/plasma"
	"github.com/specspace/plasma/protocol"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProxyConfig is a data representation of a Proxy configuration
type ProxyConfig struct {
	sync.RWMutex
	watcher *fsnotify.Watcher

	removeCallback      func()
	changeCallback      func()
	process             process.Process
	onlineStatusPacket  *protocol.Packet
	offlineStatusPacket *protocol.Packet

	DomainName        string               `json:"domainName"`
	ListenTo          string               `json:"listenTo"`
	ProxyTo           string               `json:"proxyTo"`
	ProxyProtocol     bool                 `json:"proxyProtocol"`
	Timeout           int                  `json:"timeout"`
	DisconnectMessage string               `json:"disconnectMessage"`
	Docker            DockerConfig         `json:"docker"`
	OnlineStatus      StatusConfig         `json:"onlineStatus"`
	OfflineStatus     StatusConfig         `json:"offlineStatus"`
	CallbackServer    CallbackServerConfig `json:"callbackServer"`
}

type DockerConfig struct {
	DNSServer     string `json:"dnsServer"`
	ContainerName string `json:"containerName"`
	Timeout       int    `json:"timeout"`
	Portainer     struct {
		Address    string `json:"address"`
		EndpointID string `json:"endpointId"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	} `json:"portainer"`
}

func (docker DockerConfig) IsDocker() bool {
	return docker.ContainerName != ""
}

func (docker DockerConfig) IsPortainer() bool {
	return docker.ContainerName != "" &&
		docker.Portainer.Address != "" &&
		docker.Portainer.EndpointID != ""
}

type StatusConfig struct {
	VersionName    string `json:"versionName"`
	ProtocolNumber int    `json:"protocolNumber"`
	MaxPlayers     int    `json:"maxPlayers"`
	PlayersOnline  int    `json:"playersOnline"`
	PlayerSamples  []struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
	} `json:"playerSamples"`
	IconPath string `json:"iconPath"`
	MOTD     string `json:"motd"`
}

func (status StatusConfig) StatusResponse() plasma.StatusResponse {
	var players []plasma.PlayerInfo
	for _, sample := range status.PlayerSamples {
		players = append(players, plasma.PlayerInfo{
			Name: sample.Name,
			UUID: sample.UUID,
		})
	}

	return plasma.StatusResponse{
		Version: plasma.Version{
			Name:           status.VersionName,
			ProtocolNumber: status.ProtocolNumber,
		},
		PlayersInfo: plasma.PlayersInfo{
			MaxPlayers:    status.MaxPlayers,
			PlayersOnline: status.PlayersOnline,
			Players:       players,
		},
		IconPath: status.IconPath,
		MOTD:     status.MOTD,
	}
}

type CallbackServerConfig struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

func DefaultProxyConfig() ProxyConfig {
	return ProxyConfig{
		DomainName:        "localhost",
		ListenTo:          ":25565",
		Timeout:           1000,
		DisconnectMessage: "Sorry {{username}}, but the server is offline.",
		Docker: DockerConfig{
			DNSServer: "127.0.0.11",
			Timeout:   300000,
		},
		OnlineStatus: StatusConfig{
			VersionName:    "Infrared 1.16.5",
			ProtocolNumber: 754,
			MaxPlayers:     20,
			MOTD:           "Powered by Infrared",
		},
		OfflineStatus: StatusConfig{
			VersionName:    "Infrared 1.16.5",
			ProtocolNumber: 754,
			MaxPlayers:     20,
			MOTD:           "Powered by Infrared",
		},
	}
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
// it for changes. On change the ProxyConfig will automatically LoadFromPath itself
func NewProxyConfigFromPath(path string) (*ProxyConfig, error) {
	log.Println("Loading", path)

	var cfg ProxyConfig
	if err := cfg.LoadFromPath(path); err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	cfg.watcher = watcher

	go func() {
		defer watcher.Close()
		log.Printf("Starting to watch %s", path)
		cfg.watch(path, time.Millisecond*50)
		log.Printf("Stopping to watch %s", path)
	}()

	if err := watcher.Add(path); err != nil {
		return nil, err
	}

	return &cfg, err
}

func (cfg *ProxyConfig) watch(path string, interval time.Duration) {
	// The interval protects the watcher from write event spams
	// This is necessary due to how some text editors handle file safes
	tick := time.Tick(interval)
	var lastEvent *fsnotify.Event

	for {
		select {
		case <-tick:
			if lastEvent == nil {
				continue
			}
			cfg.onConfigWrite(*lastEvent)
			lastEvent = nil
		case event, ok := <-cfg.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				cfg.removeCallback()
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				lastEvent = &event
			}
		case err, ok := <-cfg.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Failed watching %s; error %s", path, err)
		}
	}
}

func (cfg *ProxyConfig) onConfigWrite(event fsnotify.Event) {
	log.Println("Updating", event.Name)
	if err := cfg.LoadFromPath(event.Name); err != nil {
		log.Printf("Failed update on %s; error %s", event.Name, err)
		return
	}
	cfg.onlineStatusPacket = nil
	cfg.offlineStatusPacket = nil
	cfg.process = nil
	cfg.changeCallback()
}

// LoadFromPath loads the ProxyConfig from a file
func (cfg *ProxyConfig) LoadFromPath(path string) error {
	cfg.Lock()
	defer cfg.Unlock()

	var defaultCfg map[string]interface{}
	bb, err := json.Marshal(DefaultProxyConfig())
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bb, &defaultCfg); err != nil {
		return err
	}

	bb, err = ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	var loadedCfg map[string]interface{}
	if err := json.Unmarshal(bb, &loadedCfg); err != nil {
		log.Println(string(bb))
		return err
	}

	for k, v := range loadedCfg {
		defaultCfg[k] = v
	}

	bb, err = json.Marshal(defaultCfg)
	if err != nil {
		return err
	}

	return json.Unmarshal(bb, cfg)
}

func WatchProxyConfigFolder(path string, out chan *ProxyConfig) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(path); err != nil {
		return err
	}

	defer close(out)
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				proxyCfg, err := NewProxyConfigFromPath(event.Name)
				if err != nil {
					log.Printf("Failed loading %s; error %s", event.Name, err)
					continue
				}
				out <- proxyCfg
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("Failed watching %s; error %s", path, err)
		}
	}
}

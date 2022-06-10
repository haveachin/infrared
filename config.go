package infrared

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/haveachin/infrared/process"
	"github.com/haveachin/infrared/protocol"
	"github.com/haveachin/infrared/protocol/status"
)

// ProxyConfig is a data representation of a Proxy configuration
type ProxyConfig struct {
	sync.RWMutex
	watcher *fsnotify.Watcher

	removeCallback func()
	changeCallback func()
	dialer         *Dialer
	process        process.Process

	DomainName        string               `json:"domainName"`
	ListenTo          string               `json:"listenTo"`
	ProxyTo           string               `json:"proxyTo"`
	ProxyBind         string               `json:"proxyBind"`
	SpoofForcedHost   string               `json:"spoofForcedHost"`
	ProxyProtocol     bool                 `json:"proxyProtocol"`
	RealIP            bool                 `json:"realIp"`
	Timeout           int                  `json:"timeout"`
	DisconnectMessage string               `json:"disconnectMessage"`
	Docker            DockerConfig         `json:"docker"`
	OnlineStatus      StatusConfig         `json:"onlineStatus"`
	OfflineStatus     StatusConfig         `json:"offlineStatus"`
	CallbackServer    CallbackServerConfig `json:"callbackServer"`
}

func (cfg *ProxyConfig) Dialer() (*Dialer, error) {
	if cfg.dialer != nil {
		return cfg.dialer, nil
	}

	cfg.dialer = &Dialer{
		Dialer: net.Dialer{
			Timeout: time.Millisecond * time.Duration(cfg.Timeout),
			LocalAddr: &net.TCPAddr{
				IP: net.ParseIP(cfg.ProxyBind),
			},
		},
	}
	return cfg.dialer, nil
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

type PlayerSample struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

type StatusConfig struct {
	cachedPacket *protocol.Packet

	VersionName    string         `json:"versionName"`
	ProtocolNumber int            `json:"protocolNumber"`
	MaxPlayers     int            `json:"maxPlayers"`
	PlayersOnline  int            `json:"playersOnline"`
	PlayerSamples  []PlayerSample `json:"playerSamples"`
	IconPath       string         `json:"iconPath"`
	MOTD           string         `json:"motd"`
}

func (cfg StatusConfig) StatusResponsePacket() (protocol.Packet, error) {
	if cfg.cachedPacket != nil {
		return *cfg.cachedPacket, nil
	}

	var samples []status.PlayerSampleJSON
	for _, sample := range cfg.PlayerSamples {
		samples = append(samples, status.PlayerSampleJSON{
			Name: sample.Name,
			ID:   sample.UUID,
		})
	}

	responseJSON := status.ResponseJSON{
		Version: status.VersionJSON{
			Name:     cfg.VersionName,
			Protocol: cfg.ProtocolNumber,
		},
		Players: status.PlayersJSON{
			Max:    cfg.MaxPlayers,
			Online: cfg.PlayersOnline,
			Sample: samples,
		},
		Description: status.DescriptionJSON{
			Text: cfg.MOTD,
		},
	}

	if cfg.IconPath != "" {
		img64, err := loadImageAndEncodeToBase64String(cfg.IconPath)
		if err != nil {
			return protocol.Packet{}, err
		}
		responseJSON.Favicon = fmt.Sprintf("data:image/png;base64,%s", img64)
	}

	bb, err := json.Marshal(responseJSON)
	if err != nil {
		return protocol.Packet{}, err
	}

	packet := status.ClientBoundResponse{
		JSONResponse: protocol.String(bb),
	}.Marshal()

	cfg.cachedPacket = &packet
	return packet, nil
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
		OfflineStatus: StatusConfig{
			VersionName:    "Infrared 1.18",
			ProtocolNumber: 757,
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

	err := filepath.WalkDir(path, func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if dir.IsDir() {
			return nil
		}

		// check the type of file that is behind symlinks link
		if dir.Type()&os.ModeSymlink == os.ModeSymlink {
			linkedToDir, err := isLinkedToDir(path)
			if err != nil {
				return err
			}

			if linkedToDir {
				return nil
			}
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

		fullPathFile := filepath.Join(path, file.Name())

		// check the type of file that is behind symlinks link
		if file.Mode()&os.ModeSymlink == os.ModeSymlink {
			linkedToDir, err := isLinkedToDir(fullPathFile)
			if err != nil {
				return nil, err
			}

			if linkedToDir {
				continue
			}
		}

		filePaths = append(filePaths, fullPathFile)
	}

	return filePaths, err
}

func isLinkedToDir(path string) (bool, error) {
	linkedFile, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, err
	}

	linkedFileInfo, err := os.Lstat(linkedFile)
	if err != nil {
		return false, err
	}

	return linkedFileInfo.IsDir(), nil
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
	cfg.OnlineStatus.cachedPacket = nil
	cfg.OfflineStatus.cachedPacket = nil
	cfg.dialer = nil
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
				fileInfo, err := os.Lstat(event.Name)
				if err != nil {
					log.Printf("%s was created, but we failed to stat it: %v", event.Name, err)
					continue
				}

				if fileInfo.IsDir() {
					continue
				}

				// check the type of file that is behind symlinks link
				if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
					linkedToDir, err := isLinkedToDir(event.Name)
					if err != nil {
						return err
					}

					if linkedToDir {
						continue
					}
				}

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

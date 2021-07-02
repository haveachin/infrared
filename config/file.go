package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

var (
	// Isnt this the proper path to put config files into (for execution without docker)
	defaultCfgPath       = "/etc/infrared"
	defaultServerCfgPath = filepath.Join(defaultCfgPath, "configs")

	ErrFileContentIsTooSmall = errors.New("the length of the content of the file isnt enough for a minimal config")
)

func ReadServerConfigs(path string) ([]ServerConfig, error) {
	var cfgs []ServerConfig
	if path == "" {
		path = defaultServerCfgPath
	}
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
	if err != nil {
		fmt.Println(err)
		return cfgs, err
	}
	for _, filePath := range filePaths {
		log.Println("Loading", filePath)
		cfg, err := LoadServerCfgFromPath(filePath)
		if err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, cfg)
	}

	return cfgs, nil
}

func LoadServerCfgFromPath(path string) (ServerConfig, error) {
	bb, err := ioutil.ReadFile(path)
	if len(bb) < 10 {
		return ServerConfig{}, ErrFileContentIsTooSmall
	}
	if err != nil {
		return ServerConfig{}, err
	}
	return jsonToServerCfg(bb)
}

type CfgAction int

const (
	Create CfgAction = 1 + iota
	Update
	Delete
)

type ServerCfgEvent struct {
	Cfg    ServerConfig
	Action CfgAction
}

func NewCfgFileInfo(filename string, cfg ServerConfig) CfgFileInfo {
	listenAddr := ":25565"
	if cfg.ListenTo != "" {
		listenAddr = cfg.ListenTo
	}
	return CfgFileInfo{
		FileName:   filename,
		Id:         cfg.MainDomain,
		ListenAddr: listenAddr,
	}
}

type CfgFileInfo struct {
	FileName   string
	Id         string
	ListenAddr string
}

func (info CfgFileInfo) serverConfig() ServerConfig {
	return ServerConfig{
		MainDomain: info.Id,
		ListenTo:   info.ListenAddr,
	}
}

func WatchServerCfgDir(path string) (<-chan ServerCfgEvent, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(path); err != nil {
		return nil, err
	}
	eventCh := make(chan ServerCfgEvent)
	go func() {
		// Keep track of all the file names and their unique id (maindomain) and the listen to address so we can notify the manager which server needs to be removed
		//  when it files gets deleted
		fileInfo := make(map[string]CfgFileInfo)

		defer watcher.Close()
		defer close(eventCh)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				var action CfgAction
				if event.Op == fsnotify.Create {
					action = Create
				} else if event.Op == fsnotify.Write {
					action = Update
				} else if event.Op == fsnotify.Remove {
					action = Delete
				}
				if action == Create || action == Update {
					serverCfg, err := LoadServerCfgFromPath(event.Name)
					if errors.Is(err, ErrFileContentIsTooSmall) {
						continue
					} else if err != nil {
						fmt.Printf("Failed loading %s; error %s", event.Name, err)
						continue
					}
					fileInfo[event.Name] = NewCfgFileInfo(event.Name, serverCfg)
					eventCh <- ServerCfgEvent{Action: action, Cfg: serverCfg}
				} else if action == Delete {
					cfg := fileInfo[event.Name]
					eventCh <- ServerCfgEvent{
						Action: action,
						Cfg: cfg.serverConfig(),
					}
					delete(fileInfo, event.Name)
				}
			case err, ok := <-watcher.Errors:
				// Copied this from old code, not sure what to do here yet
				if !ok {
					return
				}
				fmt.Printf("Failed watching %s; error %s", path, err)
			}
		}
	}()
	return eventCh, nil
}

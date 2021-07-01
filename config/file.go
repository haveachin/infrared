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
	if len(bb) < 50 {
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

// TODO: Refactor so that deleted configes also get processed correct
//  so the loading of the config should be taken to somewhere else
//  and just send the file name and action through the channel.
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
		defer watcher.Close()
		defer close(eventCh)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				var action CfgAction
				if event.Op&fsnotify.Create == fsnotify.Create {
					action = Create
				} else if event.Op&fsnotify.Write == fsnotify.Write {
					action = Update
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					action = Delete
				}
				if action == Create || action == Update {
					serverCfg, err := LoadServerCfgFromPath(event.Name)
					if err != nil {
						fmt.Printf("Failed loading %s; error %s", event.Name, err)
						continue
					}
					eventCh <- ServerCfgEvent{Action: action, Cfg: serverCfg}
				} else if action == Delete {
					eventCh <- ServerCfgEvent{Action: action}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("Failed watching %s; error %s", path, err)
			}
		}
	}()
	return eventCh, nil
}

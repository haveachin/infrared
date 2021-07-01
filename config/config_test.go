package config_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/haveachin/infrared/config"
)

func TestLoadServerCfgFromPath(t *testing.T) {
	cfg := config.ServerConfig{
		MainDomain: "infrared",
		ProxyTo:    ":25566",
	}

	file, _ := json.MarshalIndent(cfg, "", " ")
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Error(err)
	}

	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(file); err != nil {
		t.Error(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Error(err)
	}

	loadedCfg, err := config.LoadServerCfgFromPath(tmpfile.Name())
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(cfg, loadedCfg) {
		t.Errorf("Wanted:%v \n got: %v", cfg, loadedCfg)
	}
}

func TestReadServerConfigs(t *testing.T) {
	cfg := config.ServerConfig{
		MainDomain: "infrared",
		ProxyTo:    ":25566",
	}
	tmpDir, _ := ioutil.TempDir("", "configs")
	for i := 0; i < 3; i++ {
		file, _ := json.MarshalIndent(cfg, "", " ")
		tmpfile, _ := ioutil.TempFile(tmpDir, "example")
		defer os.Remove(tmpfile.Name())
		tmpfile.Write(file)
		tmpfile.Close()
	}
	loadedCfgs, _ := config.ReadServerConfigs(tmpDir)
	for i, loadedCfg := range loadedCfgs {
		t.Log(loadedCfg)
		if !reflect.DeepEqual(cfg, loadedCfg) {
			t.Errorf("index: %d \nWanted:%v \n got: %v", i, cfg, loadedCfg)
		}
	}
	t.Fail()
}

func TestWatchConfigDir(t *testing.T) {
	cfg := config.ServerConfig{
		MainDomain: "infrared",
		ProxyTo:    ":25566",
	}
	defaultText, _ := json.MarshalIndent(cfg, "", " ")
	t.Run("Create new file", func(t *testing.T) {
		tmpDir, _ := ioutil.TempDir("", "infrared-configs")
		cfgEventCh, _ := config.WatchServerCfgDir(tmpDir)
		go func() {
			tmpfile, _ := ioutil.TempFile(tmpDir, "example")
			tmpfile.Write(defaultText)
			tmpfile.Close()
		}()
		cfgEvent, ok := <-cfgEventCh
		if !ok {
			t.Fatal("Channel closed")
		}
		if cfgEvent.Action != config.Create {
			t.Errorf("Expected Create action but got: %v", cfgEvent.Action)
		}
	})

	t.Run("Update file", func(t *testing.T) {
		tmpDir, _ := ioutil.TempDir("", "infrared-configs")
		tmpfile, _ := ioutil.TempFile(tmpDir, "example")
		cfgEventCh, _ := config.WatchServerCfgDir(tmpDir)

		go func() {
			tmpfile.Write(defaultText)
			tmpfile.Close()
		}()

		cfgEvent, ok := <-cfgEventCh
		if !ok {
			t.Fatal("Channel closed")
		}
		if cfgEvent.Action != config.Update {
			t.Errorf("Expected Create action but got: %v", cfgEvent.Action)
		}

	})

	t.Run("Delete file", func(t *testing.T) {
		tmpDir, _ := ioutil.TempDir("", "infrared-configs")
		testFile, _ := ioutil.TempFile(tmpDir, "example")
		testFile.Close()
		cfgEventCh, err := config.WatchServerCfgDir(tmpDir)
		time.Sleep(54 * time.Millisecond)
		err = os.Remove(testFile.Name())
		if err != nil {
			t.Fatalf("Failed to remove testFile: %v", err)
		}

		cfgEvent, ok := <-cfgEventCh
		if !ok {
			t.Fatal("Channel closed")
		}
		if cfgEvent.Action != config.Delete {
			t.Errorf("Expected Create action but got: %v", cfgEvent.Action)
		}
	})

}

func TestLengthSimpleConfig(t *testing.T) {
	cfg := config.ServerConfig{
		MainDomain: "infrared",
		ProxyTo:    ":25566",
	}
	defaultText, _ := json.MarshalIndent(cfg, "", " ")
	t.Log(len(defaultText))
	// t.Fail()
}

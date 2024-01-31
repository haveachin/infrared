package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	ir "github.com/haveachin/infrared/pkg/infrared"
	"gopkg.in/yaml.v3"
)

type FileType string

const (
	YAML FileType = "yaml"
)

type decoder interface {
	Decode(io.Reader, any) error
}

type decoderFunc func(io.Reader, any) error

func (fn decoderFunc) Decode(r io.Reader, v any) error {
	return fn(r, v)
}

func newYamlDecoder() decoder {
	return decoderFunc(func(r io.Reader, v any) error {
		return yaml.NewDecoder(r).Decode(v)
	})
}

// FileProvider reads a config file and returns a populated infrared.Config struct.
type FileProvider struct {
	ConfigPath string
	// Must be a directory
	ProxiesPath string
	// Types of the file
	// Defaults to YAML
	Type FileType
}

func (p FileProvider) Config() (ir.Config, error) {
	var dcr decoder
	switch p.Type {
	case YAML:
		fallthrough
	default:
		dcr = newYamlDecoder()
	}

	return p.readAndUnmashalConfig(dcr)
}

func (p FileProvider) readAndUnmashalConfig(dcr decoder) (ir.Config, error) {
	path, err := filepath.EvalSymlinks(p.ConfigPath)
	if err != nil {
		return ir.Config{}, err
	}

	f, err := os.Open(path)
	if err != nil {
		return ir.Config{}, err
	}
	defer f.Close()

	var cfg ir.Config
	if err = dcr.Decode(f, &cfg); err != nil {
		return ir.Config{}, fmt.Errorf("failed to decode file %q: %w", p.ConfigPath, err)
	}

	srvCfgs, err := loadServerConfigs(dcr, p.ProxiesPath)
	if err != nil {
		return ir.Config{}, err
	}
	cfg.ServerConfigs = srvCfgs

	return cfg, nil
}

func loadServerConfigs(dcr decoder, path string) ([]ir.ServerConfig, error) {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	if err = filepath.WalkDir(path, walkServerDirFunc(&paths)); err != nil {
		return nil, err
	}

	return readAndUnmashalServerConfigs(dcr, paths)
}

func walkServerDirFunc(paths *[]string) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Type()&os.ModeSymlink == os.ModeSymlink {
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
		}

		*paths = append(*paths, path)
		return nil
	}
}

func readAndUnmashalServerConfigs(dcr decoder, paths []string) ([]ir.ServerConfig, error) {
	cfgs := make([]ir.ServerConfig, 0)
	for _, path := range paths {
		cfg, err := readAndUnmashalServerConfig(dcr, path)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, cfg)
	}

	return cfgs, nil
}

func readAndUnmashalServerConfig(dcr decoder, path string) (ir.ServerConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return ir.ServerConfig{}, err
	}
	defer f.Close()

	cfg := ir.ServerConfig{}
	if err = dcr.Decode(f, &cfg); err != nil && !errors.Is(err, io.EOF) {
		return ir.ServerConfig{}, err
	}

	return cfg, nil
}

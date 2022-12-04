package traffic_limiter

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type storage struct {
	path  string
	mu    sync.Mutex
	cache *Bandwidth
}

func newStorage(path string) (*storage, error) {
	if err := createFileIfNotExist(path); err != nil {
		return nil, err
	}

	return &storage{
		path: path,
	}, nil
}

func createFileIfNotExist(path string) error {
	exists, err := doesFileExist(path)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return os.WriteFile(path, nil, 0644)
}

func doesFileExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

type Bandwidth struct {
	Servers map[string]BandwidthServer `yaml:"servers"`
}

type BandwidthServer struct {
	ConsumedBytes int64     `yaml:"consumedBytes"`
	LastResetAt   time.Time `yaml:"lastResetAt"`
}

func (s *storage) readBandwidth() (*Bandwidth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil {
		return s.cache, nil
	}

	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bw := &Bandwidth{}
	if err := yaml.NewDecoder(f).Decode(bw); err != nil {
		if errors.Is(err, io.EOF) {
			return &Bandwidth{
				Servers: map[string]BandwidthServer{},
			}, nil
		}
		return nil, err
	}

	return bw, nil
}

func (s *storage) writeBandwidth(bw *Bandwidth) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache = bw

	data, err := yaml.Marshal(bw)
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

func (s *storage) ConsumedBytes(serverID string) (int64, error) {
	bw, err := s.readBandwidth()
	if err != nil {
		return 0, err
	}

	return bw.Servers[serverID].ConsumedBytes, nil
}

func (s *storage) AddConsumedBytes(serverID string, consumedBytes int64) (int64, error) {
	bw, err := s.readBandwidth()
	if err != nil {
		return 0, err
	}

	srv, ok := bw.Servers[serverID]
	if !ok {
		srv = BandwidthServer{
			ConsumedBytes: consumedBytes,
		}
	} else {
		srv.ConsumedBytes += consumedBytes
	}
	bw.Servers[serverID] = srv

	if err := s.writeBandwidth(bw); err != nil {
		return 0, err
	}
	return srv.ConsumedBytes, nil
}

func (s *storage) ResetConsumedBytes(serverID string) error {
	bw, err := s.readBandwidth()
	if err != nil {
		return err
	}

	bw.Servers[serverID] = BandwidthServer{
		ConsumedBytes: 0,
		LastResetAt:   time.Now(),
	}

	return s.writeBandwidth(bw)
}

package infrared

import (
	"net"
)

type Filterer interface {
	Filter(c net.Conn) error
}

type FilterFunc func(c net.Conn) error

func (f FilterFunc) Filter(c net.Conn) error {
	return f(c)
}

type FiltersConfig struct {
	RateLimiter *RateLimiterConfig `yaml:"rateLimiter"`
}

func NewFilterConfig() FiltersConfig {
	rl := NewRateLimiterConfig()

	return FiltersConfig{
		RateLimiter: &rl,
	}
}

type Filter struct {
	cfg       FiltersConfig
	filterers []Filterer
}

func NewFilter(cfg FiltersConfig) Filter {
	filterers := make([]Filterer, 0)

	if cfg.RateLimiter != nil {
		cfg := cfg.RateLimiter
		f := RateLimitByIP(cfg.RequestLimit, cfg.WindowLength)
		filterers = append(filterers, f)
	}

	return Filter{
		cfg:       cfg,
		filterers: filterers,
	}
}

func (f Filter) Filter(c net.Conn) error {
	for _, f := range f.filterers {
		if err := f.Filter(c); err != nil {
			return err
		}
	}
	return nil
}

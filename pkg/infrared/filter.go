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

type FilterConfigFunc func(cfg *FiltersConfig)

func WithFilterConfig(c FiltersConfig) FilterConfigFunc {
	return func(cfg *FiltersConfig) {
		*cfg = c
	}
}

type FiltersConfig struct {
	RateLimiter *RateLimiterConfig `yaml:"rateLimiter"`
}

type Filter struct {
	cfg       FiltersConfig
	filterers []Filterer
}

func NewFilter(fns ...FilterConfigFunc) Filter {
	var cfg FiltersConfig
	for _, fn := range fns {
		fn(&cfg)
	}

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

package infrared

import (
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Gateway is a data structure that holds all proxies and incoming connections
type Gateway struct {
	gates map[string]*Gate
	wg    *sync.WaitGroup
}

// NewGateway creates a new gateway that orchestrates all proxies
func NewGateway(vprs []*viper.Viper) Gateway {
	g := Gateway{
		gates: map[string]*Gate{},
		wg:    &sync.WaitGroup{},
	}

	for _, vpr := range vprs {
		cfg, err := LoadConfig(vpr)
		if err != nil {
			log.Err(err).Msg("Failed to read config")
			continue
		}

		hw, err := NewHighway(cfg)
		if err != nil {
			log.Err(err).Msgf("Failed to create proxy %s[%s]", cfg.DomainName, cfg.ListenTo)
			continue
		}

		if err := g.add(hw); err != nil {
			log.Err(err).Msgf("Failed to add proxy %s[%s] to gateway", cfg.DomainName, cfg.ListenTo)
			continue
		}

		vpr.WatchConfig()
		vpr.OnConfigChange(g.onConfigChange(hw, vpr))
	}

	return g
}

// Open opens the gateway and starts all proxies
func (g *Gateway) Open() {
	if len(g.gates) <= 0 {
		log.Fatal().Msg("Gateway has no gates")
		return
	}

	log.Info().Msg("Opening gateway")

	for _, gate := range g.gates {
		loopGate := *gate
		g.wg.Add(1)
		go func() {
			log.Err(loopGate.Open())
			g.wg.Done()
		}()
	}

	g.wg.Wait()
	log.Info().Msg("Gateway closed")
}

func (g *Gateway) add(hw *Highway) error {
	gate, ok := g.gates[hw.ListenTo]
	if ok {
		return gate.add(hw)
	}

	gate, err := NewGate(hw.ListenTo)
	if err != nil {
		return err
	}

	g.gates[gate.ListensTo] = gate
	if err := gate.add(hw); err != nil {
		return err
	}

	return nil
}

func (g *Gateway) onConfigChange(hw *Highway, vpr *viper.Viper) func(fsnotify.Event) {
	return func(in fsnotify.Event) {
		log.Info().Msgf("Configuration \"%s\" changed", in.Name)

		cfg, err := LoadConfig(vpr)
		if err != nil {
			log.Err(err).Msg("Failed to read config")
			return
		}

		if cfg.ListenTo == hw.ListenTo {
			if cfg.DomainName == hw.DomainName {
				if err := hw.ApplyConfigChange(cfg); err != nil {
					log.Err(err).Msgf("Syntax error in \"%s\"", in.Name)
					return
				}
				return
			}

			gate := g.gates[hw.ListenTo]
			currentDomainName := hw.DomainName

			if err := hw.ApplyConfigChange(cfg); err != nil {
				log.Err(err).Msgf("Syntax error in \"%s\"", in.Name)
				return
			}

			gate.highways[cfg.DomainName] = hw
			delete(gate.highways, currentDomainName)

			return
		}

		gate := g.gates[hw.ListenTo]

		if err := hw.ApplyConfigChange(cfg); err != nil {
			log.Err(err).Msgf("Syntax error in \"%s\"", in.Name)
			return
		}

		g.add(hw)

		gate.remove(hw)
		if len(gate.highways) > 0 {
			return
		}
		delete(g.gates, gate.ListensTo)

		gate = g.gates[hw.ListenTo]

		g.wg.Add(1)
		go func() {
			log.Err(gate.Open())
			g.wg.Done()
		}()
	}
}

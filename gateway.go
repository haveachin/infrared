package infrared

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Gateway is a data structure that holds all proxies and incoming connections
type Gateway struct {
	info     *log.Logger
	warning  *log.Logger
	critical *log.Logger
	gates    map[string]*Gate
	wg       *sync.WaitGroup
}

// NewGateway creates a new gateway that orchestrates all proxies
func NewGateway(vprs []*viper.Viper, info, warning, critical *log.Logger) Gateway {
	g := Gateway{
		info:     info,
		warning:  warning,
		critical: critical,
		gates:    map[string]*Gate{},
		wg:       &sync.WaitGroup{},
	}

	for _, vpr := range vprs {
		cfg, err := LoadConfig(vpr)
		if err != nil {
			warning.Printf("Failed to read config: %s", err)
			continue
		}

		hw, err := NewHighway(cfg)
		if err != nil {
			warning.Printf("Failed to create proxy %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
			continue
		}

		if err := g.add(hw); err != nil {
			warning.Printf("Failed to add proxy %s[%s] to gateway: %s", cfg.DomainName, cfg.ListenTo, err)
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
		log.Println("Gateway has no gates")
		return
	}

	g.info.Println("Opening gateway")

	for _, gate := range g.gates {
		loopGate := *gate
		g.wg.Add(1)
		go func() {
			g.info.Println(loopGate.Open())
			g.wg.Done()
		}()
	}

	g.wg.Wait()
	g.info.Println("Gateway closed")
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
		g.info.Printf("Configuration \"%s\" changed", in.Name)

		cfg, err := LoadConfig(vpr)
		if err != nil {
			g.warning.Printf("Failed to read config: %s", err)
			return
		}

		if cfg.ListenTo == hw.ListenTo {
			if cfg.DomainName == hw.DomainName {
				if err := hw.ApplyConfigChange(cfg); err != nil {
					g.warning.Printf("Syntax error in \"%s\": %s", in.Name, err)
					return
				}
				return
			}

			gate := g.gates[hw.ListenTo]
			currentDomainName := hw.DomainName

			if err := hw.ApplyConfigChange(cfg); err != nil {
				g.warning.Printf("Syntax error in \"%s\": %s", in.Name, err)
				return
			}

			gate.highways[cfg.DomainName] = hw
			delete(gate.highways, currentDomainName)

			return
		}

		gate := g.gates[hw.ListenTo]

		if err := hw.ApplyConfigChange(cfg); err != nil {
			g.warning.Printf("Syntax error in \"%s\": %s", in.Name, err)
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
			g.info.Println(gate.Open())
			g.wg.Done()
		}()
	}
}

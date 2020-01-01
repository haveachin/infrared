package infrared

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/haveachin/infrared/config"
	"github.com/spf13/viper"
)

// Gateway is a data structure that holds all proxies and incoming connections
type Gateway struct {
	gates     map[string]*Gate // ListenTo
	wg        *sync.WaitGroup
	listeners map[string]chan bool
}

// NewGateway creates a new gateway that orchestrates all proxies
func NewGateway(vprs []*viper.Viper) Gateway {
	g := Gateway{
		gates:     map[string]*Gate{},
		wg:        &sync.WaitGroup{},
		listeners: map[string]chan bool{},
	}

	for _, vpr := range vprs {
		cfg, err := config.Load(vpr)
		if err != nil {
			log.Printf("Failed to read config: %s", err)
			continue
		}

		proxy, err := NewProxy(cfg)
		if err != nil {
			log.Printf("Failed to create proxy %s[%s]: %s", cfg.DomainName, cfg.ListenTo, err)
			continue
		}

		if err := g.add(proxy); err != nil {
			log.Printf("Failed to add proxy %s[%s] to gateway: %s", cfg.DomainName, cfg.ListenTo, err)
			continue
		}

		vpr.WatchConfig()
		vpr.OnConfigChange(g.onConfigChange(proxy, vpr))
	}

	return g
}

// Open opens the gateway and starts all proxies
func (g *Gateway) Open() {
	if len(g.gates) <= 0 {
		log.Println("Gateway has no gates")
		return
	}

	log.Println("Opening gateway")

	for _, gate := range g.gates {
		loopGate := *gate
		g.wg.Add(1)
		go func() {
			log.Println(loopGate.Open())
			g.wg.Done()
		}()
	}

	g.wg.Wait()
	log.Println("Gateway closed")
}

func (g *Gateway) add(proxy *Proxy) error {
	gate, ok := g.gates[proxy.ListenTo]
	if ok {
		return gate.add(proxy)
	}

	gate, err := NewGate(proxy.ListenTo)
	if err != nil {
		return err
	}

	g.gates[gate.ListensTo] = gate
	if err := gate.add(proxy); err != nil {
		return err
	}

	return nil
}

func (g *Gateway) onConfigChange(proxy *Proxy, vpr *viper.Viper) func(fsnotify.Event) {
	return func(in fsnotify.Event) {
		log.Printf("Configuration \"%s\" changed", in.Name)

		cfg, err := config.Load(vpr)
		if err != nil {
			log.Printf("Failed to read config: %s", err)
			return
		}

		if cfg.ListenTo == proxy.ListenTo {
			if cfg.DomainName == proxy.DomainName {
				if err := proxy.ApplyConfigChange(cfg); err != nil {
					log.Printf("Syntax error in \"%s\": %s", in.Name, err)
					return
				}
				return
			}

			gate := g.gates[proxy.ListenTo]
			currentDomainName := proxy.DomainName

			if err := proxy.ApplyConfigChange(cfg); err != nil {
				log.Printf("Syntax error in \"%s\": %s", in.Name, err)
				return
			}

			gate.proxies[cfg.DomainName] = proxy
			delete(gate.proxies, currentDomainName)

			return
		}

		gate := g.gates[proxy.ListenTo]

		if err := proxy.ApplyConfigChange(cfg); err != nil {
			log.Printf("Syntax error in \"%s\": %s", in.Name, err)
			return
		}

		g.add(proxy)

		gate.remove(proxy)
		if len(gate.proxies) > 0 {
			return
		}
		delete(g.gates, gate.ListensTo)

		gate = g.gates[proxy.ListenTo]

		g.wg.Add(1)
		go func() {
			log.Println(gate.Open())
			g.wg.Done()
		}()
	}
}

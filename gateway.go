package infrared

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/haveachin/infrared/config"
	"github.com/haveachin/infrared/net"
	"github.com/haveachin/infrared/net/packet"
	"github.com/spf13/viper"
)

// Gateway is a data structure that holds all proxies and incoming connections
type Gateway struct {
	Proxies   map[string]*Proxy
	Conns     []*net.Conn
	wg        *sync.WaitGroup
	listeners map[string]chan bool
}

// NewGateway creates a new gateway that orchestrates all proxies
func NewGateway(vprs []*viper.Viper) Gateway {
	g := Gateway{
		Proxies:   map[string]*Proxy{},
		Conns:     []*net.Conn{},
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

		vpr.OnConfigChange(g.onConfigChange(proxy, vpr))
	}

	return g
}

// Open opens the gateway and starts all proxies
func (g *Gateway) Open() {
	if len(g.Proxies) <= 0 {
		log.Println("Gateway has no Proxies")
		return
	}

	log.Println("Opening gateway")

	for _, proxy := range g.Proxies {
		g.serve(proxy)
	}

	g.wg.Wait()
	log.Println("Gateway closed")
}

func (g *Gateway) add(proxy *Proxy) error {
	port := strings.Split(proxy.ListenTo, ":")[1]
	domainWithPort := fmt.Sprintf("%s:%s", proxy.DomainName, port)

	if _, ok := g.Proxies[domainWithPort]; ok {
		return fmt.Errorf("[%s] is already assigned", domainWithPort)
	}

	g.Proxies[domainWithPort] = proxy
	return nil
}

func (g *Gateway) onConfigChange(proxy *Proxy, vpr *viper.Viper) func(fsnotify.Event) {
	return func(_ fsnotify.Event) {
		log.Printf("Configuration of %s[%s] changed", proxy.DomainName, proxy.ListenTo)

		cfg, err := config.Load(vpr)
		if err != nil {
			log.Printf("Failed to read config: %s", err)
			return
		}

		port := strings.Split(cfg.ListenTo, ":")[1]
		domainWithPort := fmt.Sprintf("%s:%s", cfg.DomainName, port)

		p, ok := g.Proxies[domainWithPort]
		if !ok {
			done := g.listeners[cfg.ListenTo]
			done <- true

			delete(g.Proxies, domainWithPort)
			g.Proxies[domainWithPort] = proxy

			log.Printf("Proxy %s[%s] updated to %s[%s]", proxy.DomainName, proxy.ListenTo, cfg.DomainName, cfg.ListenTo)

			proxy.OnConfigChange(cfg)
			g.serve(proxy)
			return
		}

		if proxy != p {
			log.Printf("[%s] is already assigned: skipping", domainWithPort)
			return
		}

		proxy.OnConfigChange(cfg)
	}
}

func (g *Gateway) serve(proxy *Proxy) {
	g.wg.Add(1)
	go func() {
		if err := g.listenAndServe(proxy.ListenTo); err != nil {
			log.Printf("Proxy for %s failed: %s", proxy.ListenTo, err)
		}
		g.wg.Done()
	}()
}

func (g *Gateway) listenAndServe(addr string) error {
	log.Printf("Starting to Listen on [%s]", addr)
	listener, err := net.ListenMC(addr)
	if err != nil {
		return err
	}

	done := make(chan bool)
	g.listeners[addr] = done

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-done:
				listener.Close()
				delete(g.listeners, addr)
				return nil
			default:
				log.Printf("Could not accept [%s]: %s", conn.Addr, err)
				continue
			}
		}

		g.Conns = append(g.Conns, &conn)
		go g.handleConn(&conn)
	}
}

func (g Gateway) handleConn(conn *net.Conn) {
	pk, err := conn.ReadPacket()
	if err != nil {
		log.Printf("Handshake reading failed for [%s]: %s", conn.Addr, err)
		return
	}

	handshake, err := packet.ParseSLPHandshake(pk)
	if err != nil {
		log.Printf("Handshake parsing failed for [%s]: %s", conn.Addr, err)
		return
	}

	addr := fmt.Sprintf("%s:%d", handshake.ServerAddress, handshake.ServerPort)

	proxy, ok := g.Proxies[addr]
	if !ok {
		log.Printf("[%s] requested [%s]: unknown address", conn.Addr, addr)
		return
	}

	proxy.HandleConn(conn, handshake)
}

package infrared

import (
	"fmt"
	"log"
	"strings"

	"github.com/haveachin/infrared/net"
	"github.com/haveachin/infrared/net/packet"
)

type Gate struct {
	ListensTo string
	listener  *net.Listener
	proxies   map[string]*Proxy // domain name
	close     chan bool
}

func NewGate(listenTo string) (*Gate, error) {
	listener, err := net.ListenMC(listenTo)
	if err != nil {
		return nil, err
	}

	return &Gate{
		ListensTo: listenTo,
		listener:  listener,
		proxies:   map[string]*Proxy{},
		close:     make(chan bool, 1),
	}, nil
}

func (g *Gate) add(proxy *Proxy) error {
	if _, ok := g.proxies[proxy.DomainName]; ok {
		return fmt.Errorf("%s[%s] is already in use", proxy.DomainName, g.ListensTo)
	}

	g.proxies[proxy.DomainName] = proxy
	return nil
}

func (g *Gate) remove(proxy *Proxy) {
	delete(g.proxies, proxy.DomainName)

	if len(g.proxies) > 0 {
		return
	}

	go func() {
		g.close <- true
		g.listener.Close()
	}()
}

func (g *Gate) Open() error {
	log.Printf("Gate[%s] opened", g.ListensTo)

	for {
		conn, err := g.listener.Accept()
		if err != nil {
			select {
			case <-g.close:
				return fmt.Errorf("Gate[%s] was closed", g.ListensTo)
			default:
				log.Printf("Could not accept [%s]: %s", conn.Addr, err)
				continue
			}
		}

		log.Printf("Gate[%s]: Connection accepted", g.ListensTo)

		go func() {
			if err := g.serve(&conn); err != nil {
				log.Printf("Gate[%s]: %s", g.ListensTo, err)
			}
		}()
	}
}

func (g Gate) serve(conn *net.Conn) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return fmt.Errorf("[%s] handshake reading failed; %s", conn.Addr, err)
	}

	handshake, err := packet.ParseSLPHandshake(pk)
	if err != nil {
		return fmt.Errorf("[%s] handshake parsing failed; %s", conn.Addr, err)
	}

	addr := strings.Trim(string(handshake.ServerAddress), ".")
	addrWithPort := fmt.Sprintf("%s:%d", addr, handshake.ServerPort)
	proxy, ok := g.proxies[addr]
	if !ok {
		return fmt.Errorf("[%s] requested unknown address [%s]", conn.Addr, addrWithPort)
	}

	if err := proxy.HandleConn(conn, handshake); err != nil {
		log.Printf("Gate[%s]: %s", g.ListensTo, err)
	}

	return nil
}

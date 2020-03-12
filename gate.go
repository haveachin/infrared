package infrared

import (
	"fmt"
	"log"
	"strings"

	mc "github.com/Tnze/go-mc/net"
	"github.com/haveachin/infrared/wrapper"
)

type Gate struct {
	ListensTo string
	listener  *mc.Listener
	highways  map[string]*Highway // domain name
	close     chan bool
}

func NewGate(listenTo string) (*Gate, error) {
	listener, err := mc.ListenMC(listenTo)
	if err != nil {
		return nil, err
	}

	return &Gate{
		ListensTo: listenTo,
		listener:  listener,
		highways:  map[string]*Highway{},
		close:     make(chan bool, 1),
	}, nil
}

func (g *Gate) add(hw *Highway) error {
	if _, ok := g.highways[hw.DomainName]; ok {
		return fmt.Errorf("%s[%s] is already in use", hw.DomainName, g.ListensTo)
	}

	g.highways[hw.DomainName] = hw
	return nil
}

func (g *Gate) remove(hw *Highway) {
	delete(g.highways, hw.DomainName)

	if len(g.highways) > 0 {
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
				connAddr := conn.Socket.RemoteAddr().String()
				log.Printf("Could not accept [%s]: %s", connAddr, err)
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

func (g Gate) serve(conn *mc.Conn) error {
	connAddr := conn.Socket.RemoteAddr().String()
	pk, err := conn.ReadPacket()
	if err != nil {
		return fmt.Errorf("[%s] handshake reading failed; %s", connAddr, err)
	}

	handshake, err := wrapper.ParseSLPHandshake(pk)
	if err != nil {
		return fmt.Errorf("[%s] handshake parsing failed; %s", connAddr, err)
	}

	addr := strings.Trim(string(handshake.ServerAddress), ".")
	addrWithPort := fmt.Sprintf("%s:%d", addr, handshake.ServerPort)
	proxy, ok := g.highways[addr]
	if !ok {
		return fmt.Errorf("[%s] requested unknown address [%s]", connAddr, addrWithPort)
	}

	if err := proxy.HandleConn(conn, handshake); err != nil {
		log.Printf("Gate[%s]: %s", g.ListensTo, err)
	}

	return nil
}

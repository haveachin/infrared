package gateway

import (
	"errors"
	"sync"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol/handshaking"
)

var (
	ErrCantGetHSPacket   = errors.New("cant get handshake packet from caller")
	ErrNoServerFound     = errors.New("no server was found for this request")
	ErrNotValidHandshake = errors.New("this connection didnt provide a valid handshake")
)

func CreateBasicGatewayWithStore(store ServerStore, ch <-chan connection.HSConnection) BasicGateway {
	return BasicGateway{store: store, inCh: ch}

}

type ServerData struct {
	ConnCh chan<- connection.HSConnection
}

type Gateway interface {
	HandleConnection(conn connection.HSConnection) error
}

type BasicGateway struct {
	store ServerStore
	inCh  <-chan connection.HSConnection
}

func (g *BasicGateway) Start() error {
	for {
		conn := <-g.inCh
		g.handleConnection(conn)
	}
}

func (g *BasicGateway) handleConnection(conn connection.HSConnection) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return ErrCantGetHSPacket
	}

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		return err
	}
	addr := string(hs.ServerAddress)
	serverData, ok := g.store.FindServer(addr)
	if !ok {
		// There was no server to be found
		return ErrNoServerFound
	}
	conn.SetHsPk(pk)
	conn.SetHandshake(hs)

	serverData.ConnCh <- conn

	return nil
}

type ServerStore interface {
	FindServer(addr string) (ServerData, bool)
}

type SingleServerStore struct {
	Server ServerData
}

func (store *SingleServerStore) FindServer(addr string) (ServerData, bool) {
	return store.Server, store.Server.ConnCh != nil
}

type DefaultServerStore struct {
	servers sync.Map
}

func (store *DefaultServerStore) FindServer(addr string) (ServerData, bool) {
	v, ok := store.servers.Load(addr)
	if !ok {
		// Client send an invalid address/port; we don't have a v for that address
		return ServerData{}, false
	}
	server := v.(ServerData)
	return server, true
}

func (store *DefaultServerStore) AddServer(addr string, serverData ServerData) {
	store.servers.Store(addr, serverData)
}

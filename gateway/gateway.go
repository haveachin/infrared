package gateway

import (
	"errors"
	"log"
	"sync"

	"github.com/haveachin/infrared/connection"
)

var (
	ErrNoServerFound     = errors.New("no server was found for this request")
	ErrNotValidHandshake = errors.New("this connection didnt provide a valid handshake")
)

func CreateBasicGatewayWithStore(store ServerStore, ch <-chan connection.GatewayConnection) BasicGateway {
	return BasicGateway{store: store, inCh: ch}

}

type ServerData struct {
	ConnCh chan<- connection.GatewayConnection
}

type Gateway interface {
	HandleConnection(conn connection.GatewayConnection) error
}

type BasicGateway struct {
	store ServerStore
	inCh  <-chan connection.GatewayConnection
}

func (g *BasicGateway) Start() error {
	for {
		conn := <-g.inCh
		g.handleConnection(conn)
	}
}

func (g *BasicGateway) handleConnection(conn connection.GatewayConnection) error {
	addr := conn.ServerAddr()
	log.Printf("[i] %s requests proxy for address %s", conn.RemoteAddr(), addr)
	serverData, ok := g.store.FindServer(addr)
	if !ok {
		// There was no server to be found
		return ErrNoServerFound
	}
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

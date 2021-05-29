package gateway

import (
	"errors"
	"sync"

	"github.com/haveachin/infrared/connection"
)

var (
	ErrNoServerFound     = errors.New("no server was found for this request")
	ErrNotValidHandshake = errors.New("this connection didnt provide a valid handshake")
)

func CreateBasicGatewayWithStore(store ServerStore, ch <-chan connection.Connection) BasicGateway {
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
	inCh  <-chan connection.Connection
}

func (g *BasicGateway) Start() {

	for {
		conn := <-g.inCh
		g.handleConnection(conn)
	}

}

// In this case it will be using the same server for status&login requests
// In another case they might want to use different servers for different types of requests
func (g *BasicGateway) handleConnection(conn connection.Connection) error {
	
	hsConn := conn.(connection.HSConnection)
	hs, _ := hsConn.Hs()

	serverData, ok := g.store.FindServer(string(hs.ServerAddress))
	if !ok {
		// There was no server to be found
		return ErrNoServerFound
	}
	serverData.ConnCh <- hsConn

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

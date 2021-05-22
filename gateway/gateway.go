package gateway

import (
	"sync"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/server"
)

func CreateBasicGatewayWithStore(store ServerStore) BasicGateway {
	return BasicGateway{store: store}

}

type Gateway interface {
	HandleConnection(conn connection.HSConnection)
}

type BasicGateway struct {
	store ServerStore
}

func (g *BasicGateway) Start() {

}

// In this case it will be using the same server for status&login requests
// In another case they might want to use different servers for different types of requests
func (g *BasicGateway) HandleConnection(conn connection.HSConnection) {
	targetServer, ok := g.store.FindServer(conn)
	if !ok {
		// There was no server to be found
		return
	}
	switch connection.ParseRequestType(conn) {
	case connection.LoginRequest:
		lConn := conn.(connection.LoginConnection)
		server.HandleLoginRequest(lConn, targetServer)
	case connection.StatusRequest:
		sConn := conn.(connection.StatusConnection)
		server.HandleStatusRequest(sConn, targetServer)
	}

}

type ServerStore interface {
	FindServer(conn connection.HSConnection) (server.Server, bool)
}

type SingleServerStore struct {
	Server server.Server
}

func (store *SingleServerStore) FindServer(conn connection.HSConnection) (server.Server, bool) {
	return store.Server, store.Server != nil
}

type DefaultServerStore struct {
	servers sync.Map
}

func (store *DefaultServerStore) FindServer(conn connection.HSConnection) (server.Server, bool) {
	hs, err := conn.Hs()
	if err != nil {
		return nil, false
	}
	proxyUID := hs.ParseServerAddress()
	v, ok := store.servers.Load(proxyUID)
	if !ok {
		// Client send an invalid address/port; we don't have a v for that address
		return nil, false
	}
	server := v.(server.Server)
	return server, true
}

func (store *DefaultServerStore) AddServer(addr string, server server.Server) {
	store.servers.Store(addr, server)
}

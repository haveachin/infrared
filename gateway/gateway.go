package gateway

import (
	"errors"
	"fmt"

	"github.com/haveachin/infrared/connection"
	"github.com/haveachin/infrared/protocol/handshaking"
)

var (
	ErrCantGetHSPacket   = errors.New("cant get handshake packet from caller")
	ErrNoServerFound     = errors.New("no server was found for this request")
	ErrNotValidHandshake = errors.New("this connection didnt provide a valid handshake")
)

func NewBasicGatewayWithStore(store ServerStore, connCh <-chan connection.HandshakeConn, closeCh <-chan struct{}) BasicGateway {
	return BasicGateway{store: store, inCh: connCh, closeCh: closeCh}
}

type ServerData struct {
	ConnCh chan<- connection.HandshakeConn
}

type BasicGateway struct {
	store   ServerStore
	inCh    <-chan connection.HandshakeConn
	closeCh <-chan struct{}
	// TODO: Refactor this
	Logger func(msg string)
}

func (g *BasicGateway) Start() error {
Forloop:
	for {
		select {
		case conn := <-g.inCh:
			go func() {
				err := g.handleConn(conn)
				if errors.Is(err, ErrNoServerFound) {
					// If default status is set and its a status request, send it here to the client ...?
				}
				if err != nil {
					conn.Conn().Close()
				}
			}()
		case <-g.closeCh:
			break Forloop
		}
	}
	return nil
}

func (g *BasicGateway) handleConn(conn connection.HandshakeConn) error {
	pk, err := conn.ReadPacket()
	if err != nil {
		return ErrCantGetHSPacket
	}

	hs, err := handshaking.UnmarshalServerBoundHandshake(pk)
	if err != nil {
		return err
	}
	g.Logger(fmt.Sprintf("[>] Incoming %s on listener %s", conn.RemoteAddr(), hs.ServerAddress))
	addr := string(hs.ServerAddress)
	serverData, ok := g.store.FindServer(addr)
	if !ok {
		// There was no server to be found
		g.Logger(fmt.Sprintf("[>] No server with  %s found", hs.ServerAddress))
		return ErrNoServerFound
	}
	conn.HandshakePacket = pk
	conn.Handshake = hs

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
	servers map[string]ServerData
}

func (store *DefaultServerStore) FindServer(addr string) (ServerData, bool) {
	server, ok := store.servers[addr]
	if !ok {
		// Client send an invalid address/port; we don't have a v for that address
		return ServerData{}, false
	}
	return server, true
}

func (store *DefaultServerStore) AddServer(addr string, serverData ServerData) {
	store.servers[addr] = serverData
}

func CreateDefaultServerStore() DefaultServerStore {
	return DefaultServerStore{
		servers: make(map[string]ServerData),
	}
}

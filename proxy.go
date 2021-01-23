package main

import (
	"github.com/haveachin/infrared/callback"
	"io"
	"os"
	"sync"
	"time"

	"github.com/haveachin/infrared/mc"
	"github.com/haveachin/infrared/mc/protocol"
)

// proxy is a TCP server that takes an incoming request and sends it to another
// server, proxying the response back to the client.
type proxy struct {
	// ClientBoundModifiers modify traffic that is send from the server to the client
	ClientBoundModifiers []Modifier
	// ServerBoundModifiers modify traffic that is send from the client to the server
	ServerBoundModifiers []Modifier

	config
	logger io.Writer
}

// newProxy takes a config an creates a new proxy based on it
func newProxy(cfg config) proxy {
	return proxy{
		ClientBoundModifiers: []Modifier{},
		ServerBoundModifiers: []Modifier{},
		config:               cfg,
		logger:               os.Stdout,
	}
}

func (proxy proxy) uid() string {
	return proxy.DomainName + proxy.ListenTo
}

// handleConn takes a minecraft client connection and it's initial handshake packet
// and relays all following packets to the remote connection (proxyTo)
func (proxy *proxy) handleConn(conn mc.Conn) error {
	packet, err := conn.PeekPacket()
	if err != nil {
		return err
	}

	handshake, err := protocol.ParseSLPHandshake(packet)
	if err != nil {
		return err
	}
	rconn, err := mc.DialTimeout(proxy.ProxyTo, conn.RemoteAddr(), time.Millisecond*500, proxy.ProxyProtocol)
	if err != nil {
		defer conn.Close()
		if handshake.IsStatusRequest() {
			return proxy.server.HandleConn(conn)
		}

		isProcessRunning, err := proxy.process.IsRunning()
		if err != nil {
			logger.Err(err).Interface(callback.EventKey, callback.ErrorEvent).Msg("Could not determine if the container is running")
			return proxy.server.HandleConn(conn)
		}

		if isProcessRunning {
			return proxy.server.HandleConn(conn)
		}

		logger.Info().Interface(callback.EventKey, callback.ContainerStartEvent).Msg("Starting container")
		if err := proxy.process.Start(); err != nil {
			logger.Err(err).Interface(callback.EventKey, callback.ErrorEvent).Msg("Could not start the container")
			return proxy.server.HandleConn(conn)
		}

		proxy.startTimeout()

		return proxy.server.HandleConn(conn)
	}

	if handshake.IsLoginRequest() {
		username, err := sniffUsername(conn, rconn)
		if err != nil {
			return err
		}

		proxy.stopTimeout()
		proxy.players.put(&conn, username)
		logger = logger.With().Str("username", username).Logger()
		logger.Info().Interface(callback.EventKey, callback.PlayerJoinEvent).Msgf("%s joined the game", username)

		defer func() {
			logger.Info().Interface(callback.EventKey, callback.PlayerLeaveEvent).Msgf("%s left the game", username)
			proxy.players.remove(&conn)

			if proxy.players.length() <= 0 {
				proxy.startTimeout()
			}
		}()
	}

	wg := sync.WaitGroup{}

	var pipe = func(src, dst mc.Conn, modifiers []proxy.Modifier) {
		defer wg.Done()

		buffer := make([]byte, 0xffff)

		for {
			n, err := src.Read(buffer)
			if err != nil {
				return
			}

			data := buffer[:n]

			for _, modifier := range modifiers {
				if modifier == nil {
					continue
				}

				modifier.Modify(src, dst, &data)
			}

			_, err = dst.Conn.Write(data)
			if err != nil {
				return
			}
		}
	}

	wg.Add(2)
	go pipe(conn, rconn, proxy.ClientBoundModifiers)
	go pipe(rconn, conn, proxy.ServerBoundModifiers)
	wg.Wait()

	conn.Close()
	rconn.Close()

	return nil
}

func (proxy *proxy) startTimeout() {
	if proxy.cancelTimeout != nil {
		proxy.stopTimeout()
	}

	timer := time.AfterFunc(proxy.timeout, func() {
		proxy.logger.Info().Interface(callback.EventKey, callback.ContainerStopEvent).Msgf("Stopping container")
		if err := proxy.process.Stop(); err != nil {
			proxy.logger.Err(err).Interface(callback.EventKey, callback.ErrorEvent).Msg("Failed to stop the container")
		}
	})

	proxy.cancelTimeout = func() {
		timer.Stop()
		proxy.logger.Debug().Msg("Timeout canceled")
	}

	proxy.logger.Info().Interface(callback.EventKey, callback.ContainerTimeoutEvent).Msgf("Timing out in %s", proxy.timeout)
}

func (proxy *proxy) stopTimeout() {
	if proxy.cancelTimeout == nil {
		return
	}

	proxy.cancelTimeout()
	proxy.cancelTimeout = nil
}

func (proxy *proxy) Close() {
	for _, conn := range proxy.players.keys() {
		if err := conn.Close(); err != nil {
			proxy.logger.Err(err)
		}
	}
}

func sniffUsername(conn, rconn mc.Conn) (string, error) {
	// Handshake
	packet, err := conn.ReadPacket()
	if err != nil {
		return "", err
	}

	if err := rconn.WritePacket(packet); err != nil {
		return "", err
	}

	// Login
	packet, err = conn.ReadPacket()
	if err != nil {
		return "", err
	}

	loginStartPacket, err := protocol.ParseClientLoginStart(packet)
	if err != nil {
		return "", err
	}

	if err := rconn.WritePacket(packet); err != nil {
		return "", err
	}

	return string(loginStartPacket.Name), nil
}

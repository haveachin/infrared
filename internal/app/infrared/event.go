package infrared

import "net"

const (
	NewConnEventTopic                  = "NewConn"
	PreConnProcessingEventTopic        = "PreConnProcessing"
	PostConnProcessingEventTopic       = "PostConnProcessing"
	PreServerConnConnectingEventTopic  = "PreServerConnConnecting"
	PostServerConnConnectingEventTopic = "PostServerConnConnecting"

	ClientJoinEventTopic  = "PlayerJoin"
	ClientLeaveEventTopic = "PlayerLeave"
)

type NewConnectionEvent struct {
	Gateway  Gateway
	Listener net.Listener
	Conn     net.Conn
}

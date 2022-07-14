package infrared

const (
	NewConnEventTopic                  = "NewConn"
	PreConnProcessingEventTopic        = "PreConnProcessing"
	PostConnProcessingEventTopic       = "PostConnProcessing"
	PreConnConnectingEventTopic        = "PreConnConnecting"
	PostServerConnConnectingEventTopic = "PostServerConnConnecting"

	PlayerJoinEventTopic  = "PlayerJoin"
	PlayerLeaveEventTopic = "PlayerLeave"

	ServerRegisterEventTopic = "ServerRegister"
)

type NewConnEvent struct {
	Conn Conn
}

type PreConnProcessingEvent struct {
	Conn Conn
}

type PostConnProcessingEvent struct {
	ProcessedConn ProcessedConn
}

type PreConnConnectingEvent struct {
	ProcessedConn ProcessedConn
	Server        Server
}

type PlayerJoinEvent struct {
	ProcessedConn ProcessedConn
	Server        Server
	MatchedDomain string
}

type PlayerLeaveEvent struct {
	ProcessedConn ProcessedConn
	Server        Server
	MatchedDomain string
}

type ServerRegisterEvent struct {
	Server Server
}

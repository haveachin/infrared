package infrared

import "github.com/haveachin/infrared/pkg/event"

var (
	AcceptedConnEvent       event.Event[AcceptedConnPayload]
	PreConnProcessingEvent  event.Event[PreConnProcessingPayload]
	PostConnProcessingEvent event.Event[PostConnProcessingPayload]
	PrePlayerJoinEvent      event.Event[PrePlayerJoinPayload]
	PlayerJoinEvent         event.Event[PlayerJoinPayload]
	PlayerLeaveEvent        event.Event[PlayerLeavePayload]
)

type AcceptedConnPayload struct {
	Conn Conn
}

type PreConnProcessingPayload struct {
	Conn Conn
}

type PostConnProcessingPayload struct {
	Player Player
}

type PrePlayerJoinPayload struct {
	Player        Player
	Server        Server
	MatchedDomain string
}

type PlayerJoinPayload struct {
	Player        Player
	Server        Server
	MatchedDomain string
}

type PlayerLeavePayload struct {
	Player        Player
	Server        Server
	MatchedDomain string
	ConsumedBytes int64
}

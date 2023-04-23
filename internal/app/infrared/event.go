package infrared

import (
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

const (
	AcceptedConnEventTopic     = "AcceptedConn"
	PreProcessingEventTopic    = "PreProcessing"
	PostProcessingEventTopic   = "PostProcessing"
	PrePlayerJoinEventTopic    = "PrePlayerJoin"
	PlayerJoinEventTopic       = "PlayerJoin"
	PlayerLeaveEventTopicAsync = "PlayerLeave"
)

// isEventCanceled evaluates all incoming replys and returns if the event
// is canceled and the Reply that canceled it.
func isEventCanceled(replyChan <-chan event.Reply, logger *zap.Logger) bool {
	for reply := range replyChan {
		if reply.Err == nil {
			continue
		}

		logger.Debug("event canceled",
			zap.String("reason", reply.Err.Error()),
		)

		return true
	}
	return false
}

type AcceptedConnEvent struct {
	Conn Conn
}

type PreConnProcessingEvent struct {
	Conn Conn
}

type PostConnProcessingEvent struct {
	Player Player
}

type PrePlayerJoinEvent struct {
	Player        Player
	Server        Server
	MatchedDomain string
}

type PlayerJoinEvent struct {
	Player        Player
	Server        Server
	MatchedDomain string
}

type PlayerLeaveEvent struct {
	Player        Player
	Server        Server
	MatchedDomain string
	ConsumedBytes int64
}

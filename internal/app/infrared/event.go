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

		logger.Info("event canceled",
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
	ConsumedBytes int64
}

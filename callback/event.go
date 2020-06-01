package callback

import "errors"

type Event string

const EventKey = "event"

const (
	ErrorEvent          Event = "Error"
	PlayerJoinEvent     Event = "PlayerJoin"
	PlayerLeaveEvent    Event = "PlayerLeave"
	ProcessStartEvent   Event = "ProcessStart"
	ProcessStopEvent    Event = "ProcessStop"
	ProcessTimeoutEvent Event = "ProcessTimeout"
)

var Events = []Event{
	ErrorEvent,
	PlayerJoinEvent,
	PlayerLeaveEvent,
	ProcessStartEvent,
	ProcessStopEvent,
	ProcessTimeoutEvent,
}

func ParseEvent(name string) (Event, error) {
	for _, event := range Events {
		if name == string(event) {
			return event, nil
		}
	}

	return "", errors.New("not a registered event")
}

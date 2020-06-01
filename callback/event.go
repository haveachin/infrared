package callback

import "errors"

type Event string

const (
	ErrorEvent        Event = "Error"
	PlayerJoinEvent   Event = "PlayerJoin"
	PlayerLeaveEvent  Event = "PlayerLeave"
	ProcessStartEvent Event = "ProcessStart"
	ProcessStopEvent  Event = "ProcessStop"
)

var Events = []Event{
	ErrorEvent,
	PlayerJoinEvent,
	PlayerLeaveEvent,
	ProcessStartEvent,
	ProcessStopEvent,
}

func ParseEvent(name string) (Event, error) {
	for _, event := range Events {
		if name == string(event) {
			return event, nil
		}
	}

	return "", errors.New("not a registered event")
}

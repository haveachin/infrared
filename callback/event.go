package callback

import "errors"

type Event string

const EventKey = "event"

const (
	ErrorEvent            Event = "Error"
	PlayerJoinEvent       Event = "PlayerJoin"
	PlayerLeaveEvent      Event = "PlayerLeave"
	ContainerStartEvent   Event = "ContainerStart"
	ContainerStopEvent    Event = "ContainerStop"
	ContainerTimeoutEvent Event = "ContainerTimeout"
)

var Events = []Event{
	ErrorEvent,
	PlayerJoinEvent,
	PlayerLeaveEvent,
	ContainerStartEvent,
	ContainerStopEvent,
	ContainerTimeoutEvent,
}

func ParseEvent(name string) (Event, error) {
	for _, event := range Events {
		if name == string(event) {
			return event, nil
		}
	}

	return "", errors.New("not a registered event")
}

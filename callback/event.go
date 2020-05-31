package callback

import "errors"

type Event string

const (
	PlayerJoin   Event = "PlayerJoin"
	PlayerLeave  Event = "PlayerLeave"
	ProcessStart Event = "ProcessStart"
	ProcessStop  Event = "ProcessStop"
)

var Events = []Event{
	PlayerJoin,
	PlayerLeave,
	ProcessStart,
	ProcessStop,
}

func ParseEvents(name string) (Event, error) {
	for _, event := range Events {
		if name == string(event) {
			return event, nil
		}
	}

	return "", errors.New("not a registered event")
}

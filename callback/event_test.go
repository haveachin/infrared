package callback

import "testing"

func TestErrorEvent_EventType(t *testing.T) {
	tt := []struct {
		event     Event
		eventType string
	}{
		{
			event:     ErrorEvent{},
			eventType: EventTypeError,
		},
		{
			event:     PlayerJoinEvent{},
			eventType: EventTypePlayerJoin,
		},
		{
			event:     PlayerLeaveEvent{},
			eventType: EventTypePlayerLeave,
		},
		{
			event:     ContainerStartEvent{},
			eventType: EventTypeContainerStart,
		},
		{
			event:     ContainerStopEvent{},
			eventType: EventTypeContainerStop,
		},
	}

	for _, tc := range tt {
		if tc.event.EventType() != tc.eventType {
			t.Fail()
		}
	}
}

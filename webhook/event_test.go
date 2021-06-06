package webhook_test

import (
	"github.com/haveachin/infrared/webhook"
	"testing"
)

func TestErrorEvent_EventType(t *testing.T) {
	tt := []struct {
		event     webhook.Event
		eventType string
	}{
		{
			event:     webhook.ErrorEvent{},
			eventType: webhook.EventTypeError,
		},
		{
			event:     webhook.PlayerJoinEvent{},
			eventType: webhook.EventTypePlayerJoin,
		},
		{
			event:     webhook.PlayerLeaveEvent{},
			eventType: webhook.EventTypePlayerLeave,
		},
		{
			event:     webhook.ContainerStartEvent{},
			eventType: webhook.EventTypeContainerStart,
		},
		{
			event:     webhook.ContainerStopEvent{},
			eventType: webhook.EventTypeContainerStop,
		},
	}

	for _, tc := range tt {
		if tc.event.EventType() != tc.eventType {
			t.Fail()
		}
	}
}

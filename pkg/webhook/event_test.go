package webhook_test

import (
	"testing"

	"github.com/haveachin/infrared/pkg/webhook"
)

func TestErrorEvent_EventType(t *testing.T) {
	tt := []struct {
		event     webhook.Event
		eventType string
	}{
		{
			event:     webhook.EventError{},
			eventType: webhook.EventTypeError,
		},
		{
			event:     webhook.EventPlayerJoin{},
			eventType: webhook.EventTypePlayerJoin,
		},
		{
			event:     webhook.EventPlayerLeave{},
			eventType: webhook.EventTypePlayerLeave,
		},
		{
			event:     webhook.EventContainerStart{},
			eventType: webhook.EventTypeContainerStart,
		},
		{
			event:     webhook.EventContainerStop{},
			eventType: webhook.EventTypeContainerStop,
		},
	}

	for _, tc := range tt {
		if tc.event.EventType() != tc.eventType {
			t.Fail()
		}
	}
}

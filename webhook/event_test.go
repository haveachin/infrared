package webhook_test

import (
	"github.com/haveachin/infrared/webhook"
	"testing"

)

func TestErrorEvent_EventType(t *testing.T) {
	tt := []struct {
		event     webhook.Event
		eventType string
		eventString string
	}{
		{
			event:     webhook.EventError{
				Message: "[{{eventType}}] [{{proxyUID}}] {{error}}",
				Error: "Error Message",
			},
			eventType: webhook.EventTypeError,
			eventString: "[" + webhook.EventTypeError + "] [] Error Message", 
		},
		{
			event:     webhook.EventPlayerJoin{
				Message: "[{{eventType}}] [{{proxyUID}}] {{username}}",
			},
			eventType: webhook.EventTypePlayerJoin,
			eventString: "[" + webhook.EventTypePlayerJoin + "] [] ",
		},
		{
			event:     webhook.EventPlayerLeave{
				Message: "[{{eventType}}] [{{proxyUID}}] {{username}}",
			},
			eventType: webhook.EventTypePlayerLeave,
			eventString: "[" + webhook.EventTypePlayerLeave + "] [] ",
		},
		{
			event:     webhook.EventContainerStart{
				Message: "[{{eventType}}] [{{proxyUID}}]",
			},
			eventType: webhook.EventTypeContainerStart,
			eventString: "[" + webhook.EventTypeContainerStart + "] []",
		},
		{
			event:     webhook.EventContainerStop{
				Message: "[{{eventType}}] [{{proxyUID}}]",
			},
			eventType: webhook.EventTypeContainerStop,
			eventString: "[" + webhook.EventTypeContainerStop + "] []",
		},
	}

	for _, tc := range tt {
		if tc.event.EventType() != tc.eventType {
			t.Fail()
		}
		if webhook.EventString(tc.event) != tc.eventString {
			t.Errorf("got %s, expected %s", webhook.EventString(tc.event), tc.eventString)
		}
	}
}

package webhook_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/haveachin/infrared/webhook"
	"net/http"
	"testing"
)

type mockHTTPClient struct {
	*testing.T
	targetURL    string
	expectedBody *bytes.Buffer
}

func (mock *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost {
		mock.Fail()
	}

	if req.URL.String() != mock.targetURL {
		mock.Fail()
	}

	_, err := mock.expectedBody.ReadFrom(req.Body)
	if err != nil {
		mock.Error(err)
	}

	return nil, nil
}

func TestWebhook_DispatchEvent(t *testing.T) {
	tt := []struct {
		name           string
		webhook        webhook.Webhook
		event          webhook.Event
		shouldDispatch bool
	}{
		{
			name: "WithExactlyTheAllowedEvent",
			webhook: webhook.Webhook{
				URL:    "https://example.com",
				Events: []string{webhook.EventTypeError},
			},
			event: webhook.ErrorEvent{
				Error:    "my error message",
				ProxyUID: "example.com@1.2.3.4:25565",
			},
			shouldDispatch: true,
		},
		{
			name: "WithOneOfTheAllowedEvents",
			webhook: webhook.Webhook{
				URL:    "https://example.com",
				Events: []string{webhook.EventTypePlayerJoin, webhook.EventTypePlayerLeave},
			},
			event: webhook.PlayerJoinEvent{
				Username:      "notch",
				RemoteAddress: "1.2.3.4",
				TargetAddress: "1.2.3.4",
				ProxyUID:      "example.com@1.2.3.4:25565",
			},
			shouldDispatch: true,
		},
		{
			name: "ErrorsWithOneDeniedEvent",
			webhook: webhook.Webhook{
				URL:    "https://example.com",
				Events: []string{webhook.EventTypeError, webhook.EventTypePlayerLeave},
			},
			event: webhook.PlayerJoinEvent{
				Username:      "notch",
				RemoteAddress: "1.2.3.4",
				TargetAddress: "1.2.3.4",
				ProxyUID:      "example.com@1.2.3.4:25565",
			},
			shouldDispatch: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			tc.webhook.HTTPClient = &mockHTTPClient{
				T:            t,
				targetURL:    tc.webhook.URL,
				expectedBody: &body,
			}

			eventLog, err := tc.webhook.DispatchEvent(tc.event)
			if err != nil {
				if errors.Is(err, webhook.ErrEventNotAllowed) && !tc.shouldDispatch {
					return
				}
				t.Error(err)
			}

			bb, err := json.Marshal(eventLog)
			if err != nil {
				t.Error(err)
			}

			if !bytes.Equal(body.Bytes(), bb) {
				t.Fail()
			}
		})
	}
}

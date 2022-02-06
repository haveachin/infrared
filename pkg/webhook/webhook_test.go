package webhook_test

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/haveachin/infrared/pkg/webhook"
)

var errHTTPRequestFailed = errors.New("request failed")

type mockHTTPClient struct {
	*testing.T
	targetURL         string
	expectedBody      *bytes.Buffer
	requestShouldFail bool
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

	if mock.requestShouldFail {
		return nil, errHTTPRequestFailed
	}

	return nil, nil
}

func TestWebhook_DispatchEvent(t *testing.T) {
	tt := []struct {
		name                  string
		webhook               webhook.Webhook
		event                 webhook.EventLog
		shouldDispatch        bool
		httpRequestShouldFail bool
	}{
		{
			name: "WithExactlyTheAllowedEvent",
			webhook: webhook.Webhook{
				URL:               "https://example.com",
				AllowedEventTypes: []string{"Error"},
			},
			event: webhook.EventLog{
				Type:       "Error",
				OccurredAt: time.Now(),
				Data: map[string]interface{}{
					"message": "Oops!",
				},
			},
			shouldDispatch:        true,
			httpRequestShouldFail: false,
		},
		{
			name: "WithOneOfTheAllowedEvents",
			webhook: webhook.Webhook{
				URL:               "https://example.com",
				AllowedEventTypes: []string{"PlayerJoin", "PlayerLeave"},
			},
			event: webhook.EventLog{
				Type:       "PlayerJoin",
				OccurredAt: time.Now(),
				Data: map[string]interface{}{
					"username": "notch",
				},
			},
			shouldDispatch:        true,
			httpRequestShouldFail: false,
		},
		{
			name: "ErrorsWithOneDeniedEvent",
			webhook: webhook.Webhook{
				URL:               "https://example.com",
				AllowedEventTypes: []string{"Error", "PlayerLeave"},
			},
			event: webhook.EventLog{
				Type:       "PlayerJoin",
				OccurredAt: time.Now(),
				Data: map[string]interface{}{
					"username": "notch",
				},
			},
			shouldDispatch:        false,
			httpRequestShouldFail: false,
		},
		{
			name: "ErrorsWithFailedHTTPRequest",
			webhook: webhook.Webhook{
				URL:               "https://example.com",
				AllowedEventTypes: []string{"Error"},
			},
			event: webhook.EventLog{
				Type:       "Error",
				OccurredAt: time.Now(),
				Data: map[string]interface{}{
					"message": "Oops!",
				},
			},
			shouldDispatch:        true,
			httpRequestShouldFail: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			tc.webhook.HTTPClient = &mockHTTPClient{
				T:                 t,
				targetURL:         tc.webhook.URL,
				expectedBody:      &body,
				requestShouldFail: tc.httpRequestShouldFail,
			}

			if err := tc.webhook.DispatchEvent(tc.event); err != nil {
				if errors.Is(err, webhook.ErrEventTypeNotAllowed) && !tc.shouldDispatch ||
					errors.Is(err, errHTTPRequestFailed) && tc.httpRequestShouldFail {
					return
				}
				t.Error(err)
			}
		})
	}
}

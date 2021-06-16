package webhook_test

import (
	"bytes"
	"encoding/json"
	"github.com/haveachin/infrared/webhook"
	"net/http"
	"testing"
	"time"
)

type mockHTTPClient struct {
	*testing.T
	method string
	url    string
	body   *bytes.Buffer
}

func (mock *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if req.Method != mock.method {
		mock.Fail()
	}

	if req.URL.String() != mock.url {
		mock.Fail()
	}

	_, err := mock.body.ReadFrom(req.Body)
	if err != nil {
		mock.Error(err)
	}

	return nil, nil
}

func TestWebhook_Serve(t *testing.T) {
	tt := []struct {
		webhook webhook.Webhook
		event   webhook.Event
	}{
		{
			webhook: webhook.Webhook{
				URL:    "https://example.com",
				Events: []string{webhook.EventTypeError},
			},
			event: webhook.ErrorEvent{
				Error:    "my error message",
				ProxyUID: "example.com@1.2.3.4:25565",
			},
		},
		{
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
		},
	}

	for _, tc := range tt {
		body := bytes.Buffer{}
		tc.webhook.HTTPClient = &mockHTTPClient{
			T:      t,
			method: http.MethodPost,
			url:    tc.webhook.URL,
			body:   &body,
		}

		eventCh := make(chan webhook.Event)
		eventLogCh := make(chan webhook.EventLog)
		hasStopped := make(chan bool, 1)
		go func() {
			if err := tc.webhook.Serve(eventCh, eventLogCh); err != nil {
				t.Error(err)
			}
			hasStopped <- true
		}()
		eventCh <- tc.event
		eventLog := <-eventLogCh

		bb, err := json.Marshal(eventLog)
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(body.Bytes(), bb) {
			t.Fail()
		}

		tc.webhook.Stop()

		select {
		case <-hasStopped:
			break
		case <-time.After(time.Second):
			t.Fail()
		}
	}
}

func TestWebhook_DispatchEvent(t *testing.T) {
	tt := []struct {
		webhook webhook.Webhook
		event   webhook.Event
	}{
		{
			webhook: webhook.Webhook{
				URL:    "https://example.com",
				Events: []string{webhook.EventTypeError},
			},
			event: webhook.ErrorEvent{
				Error:    "my error message",
				ProxyUID: "example.com@1.2.3.4:25565",
			},
		},
		{
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
		},
	}

	for _, tc := range tt {
		body := bytes.Buffer{}
		tc.webhook.HTTPClient = &mockHTTPClient{
			T:      t,
			method: http.MethodPost,
			url:    tc.webhook.URL,
			body:   &body,
		}

		eventLog, err := tc.webhook.DispatchEvent(tc.event)
		if err != nil {
			t.Error(err)
		}

		bb, err := json.Marshal(eventLog)
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(body.Bytes(), bb) {
			t.Fail()
		}
	}
}

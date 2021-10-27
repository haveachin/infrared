package callback

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestLogger_IsValid(t *testing.T) {
	tt := []struct {
		logger Logger
		result bool
	}{
		{
			logger: Logger{
				URL:    "something",
				Events: []string{EventTypeError, EventTypePlayerJoin, EventTypeContainerStart},
			},
			result: true,
		},
		{
			logger: Logger{
				URL:    "something",
				Events: []string{EventTypeError},
			},
			result: true,
		},
		{
			logger: Logger{
				URL: "something",
			},
			result: false,
		},
		{
			logger: Logger{
				Events: []string{EventTypeError},
			},
			result: false,
		},
		{
			logger: Logger{},
			result: false,
		},
	}

	for _, tc := range tt {
		if tc.logger.isValid() != tc.result {
			t.Fail()
		}
	}
}

func TestLogger_HasEvent(t *testing.T) {
	tt := []struct {
		logger Logger
		event  Event
		result bool
	}{
		{
			logger: Logger{
				Events: []string{EventTypeError, EventTypePlayerJoin, EventTypeContainerStart},
			},
			event:  PlayerJoinEvent{},
			result: true,
		},
		{
			logger: Logger{
				Events: []string{EventTypeError},
			},
			event:  PlayerJoinEvent{},
			result: false,
		},
	}

	for _, tc := range tt {
		if tc.logger.hasEvent(tc.event) != tc.result {
			t.Fail()
		}
	}
}

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

func TestLogger_LogEvent(t *testing.T) {
	tt := []struct {
		logger Logger
		event  Event
	}{
		{
			logger: Logger{
				URL:    "https://example.com",
				Events: []string{EventTypeError},
			},
			event: ErrorEvent{
				Error:    "my error message",
				ProxyUID: "example.com@1.2.3.4:25565",
			},
		},
		{
			logger: Logger{
				URL:    "https://example.com",
				Events: []string{EventTypePlayerJoin, EventTypePlayerLeave},
			},
			event: PlayerJoinEvent{
				Username:      "notch",
				RemoteAddress: "1.2.3.4",
				TargetAddress: "1.2.3.4",
				ProxyUID:      "example.com@1.2.3.4:25565",
			},
		},
	}

	for _, tc := range tt {
		body := bytes.Buffer{}
		tc.logger.client = &mockHTTPClient{
			T:      t,
			method: http.MethodPost,
			url:    tc.logger.URL,
			body:   &body,
		}

		eventLog, err := tc.logger.LogEvent(tc.event)
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

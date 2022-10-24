package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

var ErrEventTypeNotAllowed = errors.New("event topic not allowed")

// HTTPClient represents an interface for the Webhook to send events with.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// EventLog is the struct that will be send to the Webhook.URL
type EventLog struct {
	Topics     []string  `json:"topics"`
	OccurredAt time.Time `json:"occurredAt"`
	Data       any       `json:"data"`
}

// Webhook can send a Event via POST Request to a specified URL.
// There are two ways to use a Webhook. You can directly call
// DispatchEvent or Serve to attach a channel to the Webhook.
type Webhook struct {
	ID            string
	HTTPClient    HTTPClient
	URL           string
	AllowedTopics []string
}

// hasEvent checks if Webhook.EventTypes contain the given event's type.
func (webhook Webhook) hasEvent(e EventLog) bool {
	for _, at := range webhook.AllowedTopics {
		for _, et := range e.Topics {
			if at == et {
				return true
			}
		}
	}
	return false
}

// DispatchEvent wraps the given Event in an EventLog and marshals it into JSON
// before sending it in a POST Request to the Webhook.URL.
func (webhook Webhook) DispatchEvent(e EventLog) error {
	if !webhook.hasEvent(e) {
		return ErrEventTypeNotAllowed
	}

	bb, err := json.Marshal(e)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, webhook.URL, bytes.NewReader(bb))
	if err != nil {
		return err
	}

	resp, err := webhook.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	// We don't care about the client's response, but we should still close the client's body if it exists.
	// If not closed the underlying connection cannot be reused for further requests.
	// See https://pkg.go.dev/net/http#Client.Do for more details.
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	return nil
}

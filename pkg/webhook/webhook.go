package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

var (
	ErrEventNotAllowed = errors.New("event not allowed")
)

// HTTPClient represents an interface for the Webhook to send events with.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// EventLog is the struct that will be send to the Webhook.URL
type EventLog struct {
	EventType string    `json:"eventType"`
	Timestamp time.Time `json:"timestamp"`
	Event     Event     `json:"event"`
}

// Webhook can send a Event via POST Request to a specified URL.
// There are two ways to use a Webhook. You can directly call
// DispatchEvent or Serve to attach a channel to the Webhook.
type Webhook struct {
	ID         string
	HTTPClient HTTPClient
	URL        string
	EventTypes []string
}

// hasEvent checks if Webhook.EventTypes contain the given event's type.
func (webhook Webhook) hasEvent(event Event) bool {
	for _, eventType := range webhook.EventTypes {
		if eventType == event.EventType() {
			return true
		}
	}
	return false
}

// DispatchEvent wraps the given Event in an EventLog and marshals it into JSON
// before sending it in a POST Request to the Webhook.URL.
func (webhook Webhook) DispatchEvent(event Event) error {
	if !webhook.hasEvent(event) {
		return ErrEventNotAllowed
	}

	eventLog := EventLog{
		EventType: event.EventType(),
		Timestamp: time.Now(),
		Event:     event,
	}

	bb, err := json.Marshal(eventLog)
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

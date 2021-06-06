package webhook

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
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
// SendEvent or Start to attach a channel to the Webhook.
type Webhook struct {
	HTTPClient HTTPClient

	URL    string
	Events []string
}

// hasEvent checks if Webhook.Events contain the given event's type.
func (webhook Webhook) hasEvent(event Event) bool {
	for _, e := range webhook.Events {
		if e == event.EventType() {
			return true
		}
	}
	return false
}

// Start is a blocking function that will listen to the given channel for
// an incoming Event. This Event will then be send via Webhook.SendEvent.
func (webhook Webhook) Start(ch <-chan Event) {
	for {
		event := <-ch
		eventLog, err := webhook.SendEvent(event)
		if err != nil {
			log.Printf("Could not send %v", *eventLog)
		}
	}
}

// SendEvent wraps the given Event in an EventLog and marshals it into JSON
// before sending it in a POST Request to the Webhook.URL.
func (webhook Webhook) SendEvent(event Event) (*EventLog, error) {
	if !webhook.hasEvent(event) {
		return nil, nil
	}

	eventLog := EventLog{
		EventType: event.EventType(),
		Timestamp: time.Now(),
		Event:     event,
	}

	bb, err := json.Marshal(eventLog)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, webhook.URL, bytes.NewReader(bb))
	if err != nil {
		return nil, err
	}

	_, err = webhook.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}

	return &eventLog, nil
}

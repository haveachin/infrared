package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

var (
	ErrAlreadyStarted = errors.New("already started")
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
	stop chan bool

	HTTPClient HTTPClient
	URL        string
	Events     []string
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

// Serve is a blocking function that will listen to the given channel for
// an incoming Event. This Event will then be send via the DispatchEvent function.
// After the the Event was successfully dispatched the resulting EventLog will
// be passed into the output channel. When the Event channel is closed then this returns nil.
// Errors that happen while calling DispatchEvent will be logged.
func (webhook *Webhook) Serve(in <-chan Event, out chan<- EventLog) error {
	if webhook.stop != nil {
		return ErrAlreadyStarted
	}
	webhook.stop = make(chan bool, 1)

	for {
		select {
		case event, ok := <-in:
			if !ok {
				return nil
			}

			eventLog, err := webhook.DispatchEvent(event)
			if err != nil {
				log.Printf("[w] Could not send %v", event)
				break
			}
			out <- eventLog
		case <-webhook.stop:
			webhook.stop = nil
			return nil
		}
	}
}

// Stop signals the Webhook to end serving.
// If the Webhook wasn't serving, then this will be a no-op.
func (webhook Webhook) Stop() {
	if webhook.stop == nil {
		return
	}

	webhook.stop <- true
}

// DispatchEvent wraps the given Event in an EventLog and marshals it into JSON
// before sending it in a POST Request to the Webhook.URL.
func (webhook Webhook) DispatchEvent(event Event) (EventLog, error) {
	if !webhook.hasEvent(event) {
		return EventLog{}, nil
	}

	eventLog := EventLog{
		EventType: event.EventType(),
		Timestamp: time.Now(),
		Event:     event,
	}

	bb, err := json.Marshal(eventLog)
	if err != nil {
		return EventLog{}, err
	}

	request, err := http.NewRequest(http.MethodPost, webhook.URL, bytes.NewReader(bb))
	if err != nil {
		return EventLog{}, err
	}

	_, err = webhook.HTTPClient.Do(request)
	if err != nil {
		return EventLog{}, err
	}

	return eventLog, nil
}

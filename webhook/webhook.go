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
	ErrNotStartedYet  = errors.New("not started yet")
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
// DispatchEvent or Start to attach a channel to the Webhook.
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

// Start is a blocking function that will listen to the given channel for
// an incoming Event. This Event will then be send via Webhook.DispatchEvent.
// After the the Event was successfully dispatched the resulting EventLog will
// be send into the output channel.
// Errors will just be logged.
func (webhook *Webhook) Start(in <-chan Event, out chan<- *EventLog) error {
	if webhook.stop != nil {
		return ErrAlreadyStarted
	}
	webhook.stop = make(chan bool, 1)

	for {
		select {
		case event := <-in:
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

func (webhook Webhook) Stop() error {
	if webhook.stop == nil {
		return ErrNotStartedYet
	}

	webhook.stop <- true
	return nil
}

// DispatchEvent wraps the given Event in an EventLog and marshals it into JSON
// before sending it in a POST Request to the Webhook.URL.
func (webhook Webhook) DispatchEvent(event Event) (*EventLog, error) {
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

package callback

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// HTTPClient represents an interface for the Logger to log events with.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// EventLog
type EventLog struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

func newEventLog(event Event) EventLog {
	return EventLog{
		Event:     event.EventType(),
		Timestamp: time.Now(),
		Payload:   event,
	}
}

// Logger can post events to an http endpoint
type Logger struct {
	client HTTPClient

	URL    string
	Events []string
}

func (logger Logger) isValid() bool {
	return logger.URL != "" && len(logger.Events) > 0
}

// hasEvent checks if Logger.Events contain the given event's type.
func (logger Logger) hasEvent(event Event) bool {
	hasEvent := false
	for _, e := range logger.Events {
		if e == event.EventType() {
			hasEvent = true
			break
		}
	}
	return hasEvent
}

// LogEvent posts the given event to an http endpoint if the Logger
// holds a valid URL and the Logger.Events contains given event's type.
func (logger Logger) LogEvent(event Event) (*EventLog, error) {
	if logger.client == nil {
		logger.client = http.DefaultClient
	}

	if !logger.isValid() {
		return nil, nil
	}

	if !logger.hasEvent(event) {
		return nil, nil
	}

	eventLog := newEventLog(event)
	bb, err := json.Marshal(eventLog)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, logger.URL, bytes.NewReader(bb))
	if err != nil {
		return nil, err
	}

	_, err = logger.client.Do(request)
	if err != nil {
		return nil, err
	}

	return &eventLog, nil
}

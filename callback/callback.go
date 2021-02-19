package callback

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Logger struct {
	URL    string
	Events []string
}

func (w Logger) LogEvent(event Event) {
	if w.URL == "" || len(w.Events) < 0 {
		return
	}

	hasEvent := false
	for _, e := range w.Events {
		if e == event.EventType() {
			hasEvent = true
			break
		}
	}

	if !hasEvent {
		return
	}

	eventLog := struct {
		Event     string      `json:"event"`
		Timestamp time.Time   `json:"timestamp"`
		Payload   interface{} `json:"payload"`
	}{
		Event:     event.EventType(),
		Timestamp: time.Now(),
		Payload:   event,
	}

	bb, err := json.Marshal(eventLog)
	if err != nil {
		return
	}
	log.Println("Send", w.URL, string(bb))

	_, _ = http.Post(w.URL, "application/json", bytes.NewReader(bb))
}

package callback

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func NewLogWriter(cfg Config) (LogWriter, error) {
	var events []Event

	for _, eventString := range cfg.Events {
		event, err := ParseEvent(eventString)
		if err != nil {
			return LogWriter{}, err
		}

		events = append(events, event)
	}

	return LogWriter{
		URL:    cfg.URL,
		Events: events,
	}, nil
}

type LogWriter struct {
	URL    string
	Events []Event
}

func (w LogWriter) Write(b []byte) (int, error) {
	bb := make([]byte, len(b))
	copy(bb, b)
	go w.handleLog(bb)
	return len(b), nil
}

func (w LogWriter) handleLog(b []byte) {
	if w.URL == "" {
		return
	}

	eventJSON := struct {
		Event string `json:"event"`
	}{}

	if err := json.Unmarshal(b, &eventJSON); err != nil {
		return
	}

	if len(w.Events) > 0 {
		hasEvent := false
		for _, event := range w.Events {
			if eventJSON.Event != string(event) {
				continue
			}

			hasEvent = true
			break
		}

		if !hasEvent {
			return
		}
	}

	if err := postToURL(w.URL, b); err != nil {
	}
}

func postToURL(url string, payload []byte) error {
	_, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}

	return nil
}

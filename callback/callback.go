package callback

import (
	"bytes"
	"encoding/json"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
)

type ConsoleWriter struct {
	URL    string
	Events []Event
	zerolog.ConsoleWriter
}

func (w ConsoleWriter) Write(b []byte) (int, error) {
	go w.handleLog(b)
	return w.ConsoleWriter.Write(b)
}

func (w ConsoleWriter) handleLog(b []byte) {
	eventJSON := struct {
		Event string `json:"event"`
	}{}

	if err := json.Unmarshal(b, &eventJSON); err != nil {
		log.Err(err)
		return
	}

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

	if err := postToURL(w.URL, b); err != nil {
		log.Err(err).Str("url", w.URL)
	}
}

func postToURL(url string, payload []byte) error {
	_, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}

	return nil
}

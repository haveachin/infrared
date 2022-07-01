package event

import (
	"time"

	"github.com/gofrs/uuid"
)

type Event struct {
	ID         uuid.UUID
	OccurredAt time.Time
	Topics     []string
	Data       interface{}
}

func (e Event) hasTopic(topic string) bool {
	for _, t := range e.Topics {
		if t == topic {
			return true
		}
	}
	return false
}

type Handler func(Event)

type Channel chan Event

func New(data interface{}, topic ...string) Event {
	return Event{
		ID:         uuid.Must(uuid.NewV4()),
		OccurredAt: time.Now(),
		Topics:     topic,
		Data:       data,
	}
}

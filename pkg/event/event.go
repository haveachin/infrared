package event

import (
	"time"

	"github.com/gofrs/uuid"
)

type Event struct {
	ID         uuid.UUID
	OccurredAt time.Time
	Topic      string
	Data       interface{}
}

type Handler func(Event)

type Channel chan Event

func New(topic string, data interface{}) Event {
	return Event{
		ID:         uuid.Must(uuid.NewV4()),
		OccurredAt: time.Now(),
		Topic:      topic,
		Data:       data,
	}
}

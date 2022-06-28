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

func AddListener[T any](bus Bus, fn func(T), topics ...string) uuid.UUID {
	ch := make(Channel)
	id, _ := bus.AttachChannel(uuid.Nil, ch, topics...)

	go func() {
		for event := range ch {
			e, ok := event.Data.(T)
			if !ok {
				continue
			}

			fn(e)
		}
	}()

	return id
}

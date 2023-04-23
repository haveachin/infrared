package event

import (
	"time"

	"github.com/gofrs/uuid"
)

type Reply struct {
	Data any
	Err  error
}

type Event struct {
	ID         string
	OccurredAt time.Time
	Topics     []string
	Data       any
	replyChan  chan<- Reply
}

func (e Event) hasTopic(topic string) bool {
	for _, t := range e.Topics {
		if t == topic {
			return true
		}
	}
	return false
}

type HandlerSync interface {
	HandleSync(Event) (any, error)
}

type HandlerSyncFunc func(Event) (any, error)

func (fn HandlerSyncFunc) HandleSync(e Event) (any, error) {
	return fn(e)
}

type Handler interface {
	Handle(Event)
}

type HandlerFunc func(Event)

func (fn HandlerFunc) Handle(e Event) {
	fn(e)
}

func New(data any, topic ...string) Event {
	return Event{
		ID:         uuid.Must(uuid.NewV4()).String(),
		OccurredAt: time.Now(),
		Topics:     topic,
		Data:       data,
	}
}

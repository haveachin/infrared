package event

import (
	"github.com/gofrs/uuid"
	"sync"
	"time"
)

type Topic string

type Bus struct {
	subscriber sync.Map
}

type Event struct {
	UUID       uuid.UUID
	Topic      Topic
	Data       interface{}
	OccurredAt time.Time
}

type CallbackFunc func(Event)

func (bus *Bus) Subscribe(topic Topic, channel CallbackFunc) {
	if channel == nil {
		return
	}

	subs, ok := bus.subscriber.Load(topic)
	if !ok {
		subs = []CallbackFunc{}
	}

	subs = append(subs.([]CallbackFunc), channel)
	bus.subscriber.Store(topic, subs)
}

func (bus *Bus) Publish(topic Topic, data interface{}) {
	subs, ok := bus.subscriber.Load(topic)
	if !ok {
		return
	}

	event := Event{
		UUID:       uuid.Must(uuid.NewV4()),
		Topic:      topic,
		Data:       data,
		OccurredAt: time.Now(),
	}

	for _, sub := range subs.([]CallbackFunc) {
		sub(event)
	}
}

package event

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gofrs/uuid"
)

var DefaultBus = NewBus()

var ErrRecipientNotFound = errors.New("target recipient not found")

func IsRecipientNotFoundErr(err error) bool {
	for err != nil {
		if err == ErrRecipientNotFound {
			return true
		}
	}
	return false
}

type Event struct {
	ID        uuid.UUID
	CreatedAt time.Time
	Topic     string
	Data      map[string]interface{}
}

type Handler func(Event)

type Channel chan Event

type worker struct {
	id  uuid.UUID
	in  chan Event
	out chan Event
	fn  Handler
}

func newWorker(id uuid.UUID, fn Handler) worker {
	w := worker{
		id:  id,
		in:  make(chan Event, 100),
		out: make(chan Event),
		fn:  fn,
	}
	go w.publish()
	go w.process()
	return w
}

func (w *worker) close() {
	close(w.in)
	close(w.out)
}

func (w *worker) publish() {
	for n := range w.in {
		select {
		case w.out <- n:
		case <-time.After(time.Second * 5):
		}
	}
}

func (w *worker) process() {
	for n := range w.out {
		w.fn(n)
	}
}

func (w *worker) push(n Event) {
	select {
	case w.in <- n:
	default:
	}
}

type Bus struct {
	sync.RWMutex
	ws map[uuid.UUID]worker
}

func NewBus() *Bus {
	return &Bus{
		ws: map[uuid.UUID]worker{},
	}
}

func (b *Bus) Push(topic string, keysAndValues ...interface{}) {
	b.sendEvent(b.createEvent(topic, keysAndValues...))
}

func Push(topic string, keysAndValues ...interface{}) {
	DefaultBus.Push(topic, keysAndValues...)
}

func (b *Bus) PushTo(to uuid.UUID, topic string, keysAndValues ...interface{}) error {
	return b.sendEventTo(to, b.createEvent(topic, keysAndValues...))
}

func PushTo(to uuid.UUID, topic string, keysAndValues ...interface{}) error {
	return DefaultBus.PushTo(to, topic, keysAndValues...)
}

func (b *Bus) AttachHandler(id uuid.UUID, fn Handler) (uuid.UUID, bool) {
	if fn == nil {
		panic(fmt.Sprintf("AttachHandler called with id %q and nil handler", id))
	}

	if id == uuid.Nil {
		id = uuid.Must(uuid.NewV4())
	}

	b.Lock()
	defer b.Unlock()
	w, replaced := b.ws[id]
	if replaced {
		w.close()
	}

	b.ws[id] = newWorker(id, fn)

	return id, replaced
}

func AttachHandler(id uuid.UUID, fn Handler) (uuid.UUID, bool) {
	return DefaultBus.AttachHandler(id, fn)
}

func (b *Bus) AttachFilteredHandler(id uuid.UUID, fn Handler, topics ...string) (uuid.UUID, bool) {
	if len(topics) == 0 {
		return b.AttachHandler(id, fn)
	}
	return b.AttachHandler(id, eventFilterFunc(topics, fn))
}

func AttachFilteredHandler(id uuid.UUID, fn Handler, topics ...string) (uuid.UUID, bool) {
	return DefaultBus.AttachFilteredHandler(id, fn, topics...)
}

func (b *Bus) AttachChannel(id uuid.UUID, ch Channel) (uuid.UUID, bool) {
	if ch == nil {
		panic(fmt.Sprintf("AttachChannel called with id %q and nil channel", id))
	}
	return b.AttachHandler(id, eventChanFunc(ch))
}

func AttachChannel(id uuid.UUID, ch Channel) (uuid.UUID, bool) {
	return DefaultBus.AttachChannel(id, ch)
}

func (b *Bus) AttachFilteredChannel(id uuid.UUID, ch Channel, topics ...string) (uuid.UUID, bool) {
	if len(topics) == 0 {
		return b.AttachChannel(id, ch)
	}
	return b.AttachHandler(id, eventChanFilterFunc(topics, ch))
}

func AttachFilteredChannel(id uuid.UUID, ch Channel, topics ...string) (uuid.UUID, bool) {
	return DefaultBus.AttachFilteredChannel(id, ch, topics...)
}

func (b *Bus) DetachRecipient(id uuid.UUID) bool {
	b.Lock()
	defer b.Unlock()

	if w, ok := b.ws[id]; ok {
		go w.close()
		delete(b.ws, id)
		return ok
	}

	return false
}

func DetachRecipient(id uuid.UUID) bool {
	return DefaultBus.DetachRecipient(id)
}

func (b *Bus) DetachAllRecipients() int {
	b.Lock()
	defer b.Unlock()

	n := len(b.ws)
	for _, w := range b.ws {
		w.close()
	}
	b.ws = map[uuid.UUID]worker{}

	return n
}

func DetachAllRecipients() int {
	return DefaultBus.DetachAllRecipients()
}

func (b *Bus) createEvent(topic string, keysAndValues ...interface{}) Event {
	data := map[string]interface{}{}
	for i := 0; i < len(keysAndValues); i += 2 {
		key := keysAndValues[i].(string)
		value := keysAndValues[i+1]
		data[key] = value
	}

	return Event{
		ID:        uuid.Must(uuid.NewV4()),
		CreatedAt: time.Now(),
		Topic:     topic,
		Data:      data,
	}
}

func (b *Bus) sendEvent(e Event) {
	b.RLock()
	defer b.RUnlock()

	for _, w := range b.ws {
		w.push(e)
	}
}

func (b *Bus) sendEventTo(to uuid.UUID, e Event) error {
	b.RLock()
	defer b.RUnlock()
	if w, ok := b.ws[to]; ok {
		w.push(e)
		return nil
	}
	return ErrRecipientNotFound
}

func eventChanFunc(ch Channel) Handler {
	return func(event Event) {
		ch <- event
	}
}

func eventFilterFunc(topics []string, fn Handler) Handler {
	return func(event Event) {
		for _, topic := range topics {
			if topic == event.Topic {
				fn(event)
				return
			}
		}
	}
}

func eventChanFilterFunc(topics []string, ch Channel) Handler {
	return func(event Event) {
		for _, topic := range topics {
			if topic == event.Topic {
				ch <- event
				return
			}
		}
	}
}

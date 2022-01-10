package infrared

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gofrs/uuid"
)

var defaultBus *Bus

func init() {
	defaultBus = NewBus()
}

var ErrRecipientNotFound = errors.New("target recipient not found")

func IsRecipientNotFoundErr(err error) bool {
	for err != nil {
		if err == ErrRecipientNotFound {
			return true
		}
	}
	return false
}

// Event describes a specific event with associated data
// that gets pushed to any Handler subscribed to it's topic
type Event struct {
	ID        uuid.UUID
	CreatedAt time.Time
	Topic     string
	Data      interface{}
}

// Handler can be provided to a Bus to be called per Event
type Handler func(Event)

// Channel can be provided to a Bus to have new Events pushed to it
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

// New creates a new Bus for immediate use
func NewBus() *Bus {
	return &Bus{
		ws: map[uuid.UUID]worker{},
	}
}

// Push will immediately send a new event to all currently registered recipients
func (b *Bus) Push(topic string, data interface{}) {
	b.sendEvent(b.createEvent(topic, data))
}

func Push(topic string, data interface{}) {
	defaultBus.Push(topic, data)
}

// PushTo attempts to push an even to a specific recipient
func (b *Bus) PushTo(to uuid.UUID, topic string, data interface{}) error {
	return b.sendEventTo(to, b.createEvent(topic, data))
}

func PushTo(to uuid.UUID, topic string, data interface{}) error {
	return defaultBus.PushTo(to, topic, data)
}

// AttachHandler immediately adds the provided fn to the list of recipients for new events.
//
// It will:
// - panic if fn is nil
// - generate random ID if provided ID is empty
// - return "true" if there was an existing recipient with the same identifier
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
	return defaultBus.AttachHandler(id, fn)
}

// AttachFilteredHandler attaches a handler that will only be called when events are published to specific topics
func (b *Bus) AttachFilteredHandler(id uuid.UUID, fn Handler, topics ...string) (uuid.UUID, bool) {
	if len(topics) == 0 {
		return b.AttachHandler(id, fn)
	}
	return b.AttachHandler(id, eventFilterFunc(topics, fn))
}

func AttachFilteredHandler(id uuid.UUID, fn Handler, topics ...string) (uuid.UUID, bool) {
	return defaultBus.AttachFilteredHandler(id, fn, topics...)
}

// AttachChannel immediately adds the provided channel to the list of recipients for new
// events.
//
// It will:
// - panic if ch is nil
// - generate random ID if provided ID is empty
// - return "true" if there was an existing recipient with the same identifier
func (b *Bus) AttachChannel(id uuid.UUID, ch Channel) (uuid.UUID, bool) {
	if ch == nil {
		panic(fmt.Sprintf("AttachChannel called with id %q and nil channel", id))
	}
	return b.AttachHandler(id, eventChanFunc(ch))
}

func AttachChannel(id uuid.UUID, ch Channel) (uuid.UUID, bool) {
	return defaultBus.AttachChannel(id, ch)
}

// AttachFilteredChannel attaches a channel will only have events pushed to it when they are published to specific
// topics
func (b *Bus) AttachFilteredChannel(id uuid.UUID, ch Channel, topics ...string) (uuid.UUID, bool) {
	if len(topics) == 0 {
		return b.AttachChannel(id, ch)
	}
	return b.AttachHandler(id, eventChanFilterFunc(topics, ch))
}

func AttachFilteredChannel(id uuid.UUID, ch Channel, topics ...string) (uuid.UUID, bool) {
	return defaultBus.AttachFilteredChannel(id, ch, topics...)
}

// DetachRecipient immediately removes the provided recipient from receiving any new events,
// returning true if a recipient was found with the provided id
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
	return defaultBus.DetachRecipient(id)
}

// DetachAllRecipients immediately clears all attached recipients, returning the count of those previously
// attached.
func (b *Bus) DetachAllRecipients() int {
	b.Lock()
	defer b.Unlock()

	// count how many are in there right now
	n := len(b.ws)

	for _, w := range b.ws {
		w.close()
	}

	b.ws = map[uuid.UUID]worker{}

	return n
}

func DetachAllRecipients() int {
	return defaultBus.DetachAllRecipients()
}

func (b *Bus) createEvent(t string, d interface{}) Event {
	return Event{
		ID:        uuid.Must(uuid.NewV4()),
		CreatedAt: time.Now(),
		Topic:     t,
		Data:      d,
	}
}

// sendEvent immediately calls each handler with the new event
func (b *Bus) sendEvent(ev Event) {
	b.RLock()
	defer b.RUnlock()

	for _, w := range b.ws {
		w.push(ev)
	}
}

func (b *Bus) sendEventTo(to uuid.UUID, ev Event) error {
	b.RLock()
	defer b.RUnlock()
	if w, ok := b.ws[to]; ok {
		w.push(ev)
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

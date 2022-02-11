package event

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gofrs/uuid"
)

var DefaultBus = NewInternalBus()

var ErrRecipientNotFound = errors.New("target recipient not found")

type Bus interface {
	Push(topic string, data interface{})
	PushTo(receiverId uuid.UUID, topic string, data interface{}) error
	AttachHandler(id uuid.UUID, fn Handler, topics ...string) (handlerId uuid.UUID, replaced bool)
	AttachChannel(id uuid.UUID, ch Channel, topics ...string) (channelId uuid.UUID, replaced bool)
	DetachRecipient(id uuid.UUID) (success bool)
	DetachAllRecipients() (n int)
}

type internalBus struct {
	sync.RWMutex
	ws map[uuid.UUID]worker
}

func NewInternalBus() Bus {
	return &internalBus{
		ws: map[uuid.UUID]worker{},
	}
}

func (b *internalBus) Push(topic string, data interface{}) {
	b.sendEvent(New(topic, data))
}

func Push(topic string, data interface{}) {
	DefaultBus.Push(topic, data)
}

func (b *internalBus) PushTo(to uuid.UUID, topic string, data interface{}) error {
	return b.sendEventTo(to, New(topic, data))
}

func PushTo(to uuid.UUID, topic string, data interface{}) error {
	return DefaultBus.PushTo(to, topic, data)
}

func (b *internalBus) AttachHandler(id uuid.UUID, fn Handler, topics ...string) (uuid.UUID, bool) {
	if fn == nil {
		println()
		panic(fmt.Sprintf("AttachHandler called with id %q and nil handler", id))
	}

	if len(topics) > 0 {
		fn = eventFilterFunc(topics, fn)
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

func AttachHandler(id uuid.UUID, fn Handler, topics ...string) (uuid.UUID, bool) {
	return DefaultBus.AttachHandler(id, fn, topics...)
}

func (b *internalBus) AttachChannel(id uuid.UUID, ch Channel, topics ...string) (uuid.UUID, bool) {
	return b.AttachHandler(id, eventChanFunc(ch), topics...)
}

func AttachChannel(id uuid.UUID, ch Channel, topics ...string) (uuid.UUID, bool) {
	return DefaultBus.AttachChannel(id, ch, topics...)
}

func (b *internalBus) DetachRecipient(id uuid.UUID) bool {
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

func (b *internalBus) DetachAllRecipients() int {
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

func (b *internalBus) sendEvent(e Event) {
	b.RLock()
	defer b.RUnlock()
	for _, w := range b.ws {
		w.push(e)
	}
}

func (b *internalBus) sendEventTo(to uuid.UUID, e Event) error {
	b.RLock()
	defer b.RUnlock()
	if w, ok := b.ws[to]; ok {
		w.push(e)
		return nil
	}
	return ErrRecipientNotFound
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

func eventChanFunc(ch Channel) Handler {
	return func(event Event) {
		ch <- event
	}
}

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

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

// Bus is an event bus system that notifies all it's attached recipients of pushed events.
// Recipients can be attached via a event.Handler func or an event.Channel.
type Bus interface {
	// Push pushes an event with arbitrary data to the event bus.
	Push(data any, topic ...string)
	PushTo(to string, data any, topic ...string) error
	Request(data any, topics ...string) <-chan Reply
	RequestFrom(to string, data any, topics ...string) (<-chan Reply, error)
	AttachHandler(id string, h HandlerSync, topics ...string) (handlerID string, replaced bool)
	AttachHandlerFunc(id string, fn HandlerSyncFunc, topics ...string) (handlerID string, replaced bool)
	AttachHandlerAsync(id string, h Handler, topics ...string) (handlerID string, replaced bool)
	AttachHandlerAsyncFunc(id string, fh HandlerFunc, topics ...string) (handlerID string, replaced bool)
	DetachRecipient(id string) (success bool)
	DetachAllRecipients() (n int)
}

func Push(data any, topics ...string) {
	DefaultBus.Push(data, topics...)
}

func PushTo(to string, data any, topics ...string) error {
	return DefaultBus.PushTo(to, data, topics...)
}

func Request(data any, topics ...string) <-chan Reply {
	return DefaultBus.Request(data, topics...)
}

func RequestFrom(to string, data any, topics ...string) (<-chan Reply, error) {
	return DefaultBus.RequestFrom(to, data, topics...)
}

func AttachHandler(id string, h HandlerSync, topics ...string) (string, bool) {
	return DefaultBus.AttachHandler(id, h, topics...)
}

func AttachHandlerFunc(id string, h HandlerSyncFunc, topics ...string) (string, bool) {
	return DefaultBus.AttachHandlerFunc(id, h, topics...)
}

func AttachHandlerAsync(id string, h Handler, topics ...string) (string, bool) {
	return DefaultBus.AttachHandlerAsync(id, h, topics...)
}

func AttachHandlerAsyncFunc(id string, fn HandlerFunc, topics ...string) (string, bool) {
	return DefaultBus.AttachHandlerAsync(id, fn, topics...)
}

func DetachRecipient(id string) bool {
	return DefaultBus.DetachRecipient(id)
}

func DetachAllRecipients() int {
	return DefaultBus.DetachAllRecipients()
}

type internalBus struct {
	sync.RWMutex
	ws map[string]worker
}

func NewInternalBus() Bus {
	return &internalBus{
		ws: map[string]worker{},
	}
}

func (b *internalBus) Push(data any, topics ...string) {
	b.sendEvent(New(data, topics...))
}

func (b *internalBus) PushTo(to string, data any, topics ...string) error {
	return b.sendEventTo(to, New(data, topics...))
}

func (b *internalBus) Request(data any, topics ...string) <-chan Reply {
	replyChan, _ := b.doRequest("", data, topics...)
	return replyChan
}

func (b *internalBus) RequestFrom(to string, data any, topics ...string) (<-chan Reply, error) {
	return b.doRequest(to, data, topics...)
}

func (b *internalBus) AttachHandler(id string, h HandlerSync, topics ...string) (string, bool) {
	return b.attachHandler(id, HandlerFunc(func(e Event) {
		if e.replyChan == nil {
			panic("attached to async topic")
		}

		data, err := h.HandleSync(e)
		e.replyChan <- Reply{
			EventID:   e.ID,
			HandlerID: id,
			Data:      data,
			Err:       err,
		}
	}), topics)
}

func (b *internalBus) AttachHandlerFunc(id string, fn HandlerSyncFunc, topics ...string) (string, bool) {
	return b.AttachHandler(id, fn, topics...)
}

func (b *internalBus) AttachHandlerAsync(id string, h Handler, topics ...string) (string, bool) {
	return b.attachHandler(id, HandlerFunc(func(e Event) {
		if e.replyChan != nil {
			e.replyChan <- Reply{}
		}

		h.Handle(e)
	}), topics)
}

func (b *internalBus) AttachHandlerAsyncFunc(id string, fn HandlerFunc, topics ...string) (string, bool) {
	return b.AttachHandlerAsync(id, fn, topics...)
}

func (b *internalBus) DetachRecipient(id string) bool {
	b.Lock()
	defer b.Unlock()

	if w, ok := b.ws[id]; ok {
		w.close()
		delete(b.ws, id)
		return ok
	}

	return false
}

func (b *internalBus) DetachAllRecipients() int {
	b.Lock()
	defer b.Unlock()

	n := len(b.ws)
	for _, w := range b.ws {
		w.close()
	}
	b.ws = map[string]worker{}

	return n
}

func (b *internalBus) sendEvent(e Event) int {
	b.RLock()
	defer b.RUnlock()
	for _, w := range b.ws {
		w.push(e)
	}
	return len(b.ws)
}

func (b *internalBus) sendEventTo(to string, e Event) error {
	b.RLock()
	defer b.RUnlock()
	if w, ok := b.ws[to]; ok {
		w.push(e)
		return nil
	}
	return ErrRecipientNotFound
}

func (b *internalBus) attachHandler(id string, h Handler, topics []string) (string, bool) {
	if h == nil {
		panic(fmt.Sprintf("AttachHandler called with id %q and nil handler", id))
	}

	if topics != nil {
		h = topicFilterFunc(topics, h)
	}

	if id == "" {
		id = uuid.Must(uuid.NewV4()).String()
	}

	b.Lock()
	defer b.Unlock()
	w, replaced := b.ws[id]
	if replaced {
		w.close()
	}

	b.ws[id] = newWorker(id, h)

	return id, replaced
}

func (b *internalBus) doRequest(to string, data any, topics ...string) (<-chan Reply, error) {
	bufChan := make(chan Reply)
	e := New(data, topics...)
	e.replyChan = bufChan

	n := 1
	if to == "" {
		n = b.sendEvent(e)
	} else if err := b.sendEventTo(to, e); err != nil {
		close(bufChan)
		return nil, err
	}

	replyChan := make(chan Reply)
	go func() {
		for i := 0; i < n; i++ {
			replyChan <- <-bufChan
		}
		close(bufChan)
		close(replyChan)
	}()

	return replyChan, nil
}

func topicFilterFunc(topics []string, h Handler) Handler {
	if len(topics) == 1 {
		topic := topics[0]
		return HandlerFunc(func(e Event) {
			if e.hasTopic(topic) {
				h.Handle(e)
				return
			}

			if e.replyChan != nil {
				e.replyChan <- Reply{}
			}
		})
	}

	return HandlerFunc(func(e Event) {
		for _, topic := range topics {
			if e.hasTopic(topic) {
				h.Handle(e)
				return
			}
		}

		if e.replyChan != nil {
			e.replyChan <- Reply{}
		}
	})
}

type worker struct {
	id  string
	in  chan Event
	out chan Event
	h   Handler
}

func newWorker(id string, h Handler) worker {
	w := worker{
		id:  id,
		in:  make(chan Event, 100),
		out: make(chan Event),
		h:   h,
	}
	go w.publish()
	go w.process()
	return w
}

func (w *worker) close() {
	close(w.in)
	for range w.in {
	}
}

func (w *worker) publish() {
	for e := range w.in {
		deadline := time.NewTimer(time.Second * 5)
		select {
		case w.out <- e:
			if !deadline.Stop() {
				<-deadline.C
			}
		case <-deadline.C:
		}
	}
}

func (w *worker) process() {
	for e := range w.out {
		w.h.Handle(e)
	}
	close(w.out)
}

func (w *worker) push(e Event) {
	select {
	case w.in <- e:
	default:
	}
}

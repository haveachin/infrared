package event

import (
	"errors"
	"sync"
)

type Handler[Payload any] interface {
	Handle(p Payload) (ok bool)
}

type HandlerFunc[Payload any] func(p Payload) (ok bool)

func (fn HandlerFunc[Payload]) Handle(p Payload) (ok bool) {
	return fn(p)
}

type Event[Payload any] struct {
	sync.Mutex
	handlers []HandlerFunc[Payload]
}

func (e *Event[Payload]) Handle(h Handler[Payload]) {
	e.HandleFunc(h.Handle)
}

func (e *Event[Payload]) HandleFunc(fn HandlerFunc[Payload]) {
	e.Lock()
	defer e.Unlock()

	if e.handlers == nil {
		e.handlers = []HandlerFunc[Payload]{fn}
		return
	}

	e.handlers = append(e.handlers, fn)
}

func (e *Event[Payload]) Push(p Payload) error {
	e.Lock()
	defer e.Unlock()

	if e.handlers == nil {
		return nil
	}

	for _, h := range e.handlers {
		if ok := h(p); !ok {
			return errors.New("event cancled")
		}
	}
	return nil
}

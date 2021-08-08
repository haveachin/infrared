package webhook

import (
	"time"
)

const (
	EventTypeError          string = "Error"
	EventTypePlayerJoin     string = "PlayerJoin"
	EventTypePlayerLeave    string = "PlayerLeave"
	EventTypeContainerStart string = "ContainerStart"
	EventTypeContainerStop  string = "ContainerStop"
)

type Event interface {
	EventType() string
	EventTemplate() map[string]string
	EventBaseMessage() string
}

type EventError struct {
	Error    string `json:"error"`
	ProxyUID string `json:"proxyUid"`
	Message string
}

func (event EventError) EventType() string {
	return EventTypeError
}

func (event EventError) EventTemplate() map[string]string {
	return map[string]string{
		"now":       time.Now().Format(time.RFC822),
		"eventType": event.EventType(),
		"proxyUID":  event.ProxyUID,
		"error":	 event.Error,
	}
}

func (event EventError) EventBaseMessage() string {
	return event.Message
}

type EventPlayerJoin struct {
	Username      string `json:"username"`
	RemoteAddress string `json:"remoteAddress"`
	TargetAddress string `json:"targetAddress"`
	ProxyUID      string `json:"proxyUid"`
	Message string
}

func (event EventPlayerJoin) EventType() string {
	return EventTypePlayerJoin
}

func (event EventPlayerJoin) EventTemplate() map[string]string {
	return map[string]string{
		"now":       time.Now().Format(time.RFC822),
		"eventType": event.EventType(),
		"proxyUID":  event.ProxyUID,
		"username": event.Username,
		"remoteAddress": event.RemoteAddress,
		"targetAddress": event.TargetAddress,
	}
}

func (event EventPlayerJoin) EventBaseMessage() string {
	return event.Message
}

type EventPlayerLeave struct {
	Username      string `json:"username"`
	RemoteAddress string `json:"remoteAddress"`
	TargetAddress string `json:"targetAddress"`
	ProxyUID      string `json:"proxyUid"`
	Message string
}

func (event EventPlayerLeave) EventType() string {
	return EventTypePlayerLeave
}

func (event EventPlayerLeave) EventTemplate() map[string]string {
	return map[string]string{
		"now":       time.Now().Format(time.RFC822),
		"eventType": event.EventType(),
		"proxyUID":  event.ProxyUID,
		"username": event.Username,
		"remoteAddress": event.RemoteAddress,
		"targetAddress": event.TargetAddress,
	}
}

func (event EventPlayerLeave) EventBaseMessage() string {
	return event.Message
}

type EventContainerStart struct {
	ProxyUID string `json:"proxyUid"`
	Message string
}

func (event EventContainerStart) EventType() string {
	return EventTypeContainerStart
}

func (event EventContainerStart) EventTemplate() map[string]string {
	return map[string]string{
		"now":       time.Now().Format(time.RFC822),
		"eventType": event.EventType(),
		"proxyUID":  event.ProxyUID,
	}
}

func (event EventContainerStart) EventBaseMessage() string {
	return event.Message
}

type EventContainerStop struct {
	ProxyUID string `json:"proxyUid"`
	Message string
}

func (event EventContainerStop) EventType() string {
	return EventTypeContainerStop
}

func (event EventContainerStop) EventTemplate() map[string]string {
	return map[string]string{
		"now":       time.Now().Format(time.RFC822),
		"eventType": event.EventType(),
		"proxyUID":  event.ProxyUID,
	}
}

func (event EventContainerStop) EventBaseMessage() string {
	return event.Message
}
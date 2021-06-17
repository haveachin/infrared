package webhook

const (
	EventTypeError          string = "Error"
	EventTypePlayerJoin     string = "PlayerJoin"
	EventTypePlayerLeave    string = "PlayerLeave"
	EventTypeContainerStart string = "ContainerStart"
	EventTypeContainerStop  string = "ContainerStop"
)

type Event interface {
	EventType() string
}

type EventError struct {
	Error    string `json:"error"`
	ProxyUID string `json:"proxyUid"`
}

func (event EventError) EventType() string {
	return EventTypeError
}

type EventPlayerJoin struct {
	Username      string `json:"username"`
	RemoteAddress string `json:"remoteAddress"`
	TargetAddress string `json:"targetAddress"`
	ProxyUID      string `json:"proxyUid"`
}

func (event EventPlayerJoin) EventType() string {
	return EventTypePlayerJoin
}

type EventPlayerLeave struct {
	Username      string `json:"username"`
	RemoteAddress string `json:"remoteAddress"`
	TargetAddress string `json:"targetAddress"`
	ProxyUID      string `json:"proxyUid"`
}

func (event EventPlayerLeave) EventType() string {
	return EventTypePlayerLeave
}

type EventContainerStart struct {
	ProxyUID string `json:"proxyUid"`
}

func (event EventContainerStart) EventType() string {
	return EventTypeContainerStart
}

type EventContainerStop struct {
	ProxyUID string `json:"proxyUid"`
}

func (event EventContainerStop) EventType() string {
	return EventTypeContainerStop
}

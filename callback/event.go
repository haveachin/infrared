package callback

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

type ErrorEvent struct {
	Error    string `json:"error"`
	ProxyUID string `json:"proxyUid"`
}

func (event ErrorEvent) EventType() string {
	return EventTypeError
}

type PlayerJoinEvent struct {
	Username      string `json:"username"`
	RemoteAddress string `json:"remoteAddress"`
	TargetAddress string `json:"targetAddress"`
	ProxyUID      string `json:"proxyUid"`
}

func (event PlayerJoinEvent) EventType() string {
	return EventTypePlayerJoin
}

type PlayerLeaveEvent struct {
	Username      string `json:"username"`
	RemoteAddress string `json:"remoteAddress"`
	TargetAddress string `json:"targetAddress"`
	ProxyUID      string `json:"proxyUid"`
}

func (event PlayerLeaveEvent) EventType() string {
	return EventTypePlayerLeave
}

type ContainerStartEvent struct {
	ProxyUID string `json:"proxyUid"`
}

func (event ContainerStartEvent) EventType() string {
	return EventTypeContainerStart
}

type ContainerStopEvent struct {
	ProxyUID string `json:"proxyUid"`
}

func (event ContainerStopEvent) EventType() string {
	return EventTypeContainerStop
}

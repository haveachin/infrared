package infrared

import (
	"github.com/haveachin/infrared/event"
)

const (
	EventProxyConfigChange event.Topic = iota
	EventProxyConfigRemoved
)

type ProxyConfigChangeEvent struct {
	OldConfig ProxyConfig
	NewConfig ProxyConfig
}

type ProxyConfigRemovedEvent struct {
	Config ProxyConfig
}

var eventBus = event.Bus{}

func SubscribeEvent(topic event.Topic, channel event.Channel) {
	eventBus.Subscribe(topic, channel)
}

func PublishEvent(topic event.Topic, data interface{}) {
	eventBus.Publish(topic, data)
}

package main

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/event"
)

// This function creates our plugin.
// We also declare this function with a return type of infrared.Plugin to ensure
// that we have implemented the Plugin interface correctly.
func New() infrared.Plugin {
	return &greeterPlugin{}
}

// This is our main plugin class that will implement the infrared.Plugin interface.
type greeterPlugin struct {
	// logr.Logger is the logger that infrared is using.
	// You can of course implement logging in your plugin however you like it,
	// but you are able to use the same logger as Infrared to keep consistency.
	log logr.Logger
	// This is the event bus that Infrared will use to push all occuring events to.
	eb event.Bus

	ec map[uuid.UUID]event.Channel
}

func (p greeterPlugin) Name() string {
	return "GreeterPlugin"
}

func (p greeterPlugin) Version() string {
	return "1.0.0"
}

// This is called once right before the proxies are started.
func (p *greeterPlugin) Enable(log logr.Logger, eb event.Bus) error {
	// We safe these in our struct for later use.
	p.log = log
	p.eb = eb
	// Create a new map of event channels
	p.ec = map[uuid.UUID]event.Channel{}

	// This will create a channel for events with a capacity of 10.
	// Capacity is like a buffer size, meaning the amount of events
	// we can hold in this channel. If your buffer is full then the event bus
	// will give us a grace period of 5 seconds before we start dropping events.
	playerJoinChannel := make(event.Channel, 10)
	// This attaches your playerJoinChannel to the event bus and filters for client join events.
	// It also takes an UUID as parameter that would allow us to replace an event channel that is
	// registered in that event bus under that id.
	// If we supply uuid.Nil as UUID it generate a new uuid for us, that we can use later to remove
	// the event channel again.
	id, _ := eb.AttachChannel(uuid.Nil, playerJoinChannel, infrared.ClientJoinEventTopic)
	// Now we safe the event channel with the generared UUID
	p.ec[id] = playerJoinChannel

	go p.greetPlayer(playerJoinChannel)

	// Since we don't have any errors, return nil
	return nil
}

func (p greeterPlugin) Disable() error {
	// This will detach all of our registered event channels again.
	for id, c := range p.ec {
		p.eb.DetachRecipient(id)
		// We also close all of our channels to free allocated memory
		close(c)
	}

	return nil
}

func (p greeterPlugin) greetPlayer(ch event.Channel) {
	for e := range ch {
		username := e.ID //e.Data["username"].(string)
		greeting := fmt.Sprintf("Hello, %s", username)
		p.log.Info(greeting)
	}
}

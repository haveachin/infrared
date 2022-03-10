package webhook

import (
	"errors"

	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/haveachin/infrared/pkg/webhook"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Plugin struct {
	Viper *viper.Viper

	log      *zap.Logger
	eventBus event.Bus
	eventChs map[uuid.UUID]event.Channel
	// GatewayID mapped to webhooks
	javaWhks    map[string][]webhook.Webhook
	bedrockWhks map[string][]webhook.Webhook
}

func (p Plugin) Name() string {
	return "Webhook Plugin"
}

func (p Plugin) Version() string {
	return "internal"
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.log = api.Logger()
	p.eventBus = api.EventBus()
	p.eventChs = map[uuid.UUID]event.Channel{}

	var err error
	p.javaWhks, err = p.loadWebhooks("java")
	if err != nil {
		return err
	}
	p.bedrockWhks, err = p.loadWebhooks("bedrock")
	if err != nil {
		return err
	}

	ch := make(event.Channel, 10)
	id, _ := p.eventBus.AttachChannel(uuid.Nil, ch)
	p.eventChs[id] = ch

	go p.start(ch)

	return nil
}

func (p Plugin) Disable() error {
	for id := range p.eventChs {
		p.eventBus.DetachRecipient(id)
	}

	return nil
}

func (p Plugin) start(ch event.Channel) {
	for e := range ch {
		p.handleEvent(e)
	}
}

type eventData struct {
	Edition        string `json:"edition"`
	Network        string `json:"network"`
	LocalAddr      string `json:"localAddress"`
	RemoteAddr     string `json:"remoteAddress"`
	GatewayID      string `json:"gatewayId"`
	ServerAddr     string `json:"serverAddress,omitempty"`
	Username       string `json:"username,omitempty"`
	IsLoginRequest *bool  `json:"isLoginRequest,omitempty"`
	ServerID       string `json:"serverId,omitempty"`

	edition infrared.Edition
}

func mapConn(data *eventData, c infrared.Conn) {
	data.edition = c.Edition()
	data.Edition = c.Edition().String()
	data.Network = c.LocalAddr().Network()
	data.LocalAddr = c.LocalAddr().String()
	data.RemoteAddr = c.RemoteAddr().String()
	data.GatewayID = c.GatewayID()
}

func mapProcessedConn(data *eventData, pc infrared.ProcessedConn) {
	mapConn(data, pc)
	data.ServerAddr = pc.ServerAddr()
	data.Username = pc.Username()
	var isLoginRequest = pc.IsLoginRequest()
	data.IsLoginRequest = &isLoginRequest
}

func mapServer(data *eventData, s infrared.Server) {
	data.ServerID = s.ID()
}

func (p Plugin) handleEvent(e event.Event) {
	var data eventData
	switch e := e.Data.(type) {
	case infrared.NewConnEvent:
		mapConn(&data, e.Conn)
	case infrared.PreConnProcessingEvent:
		mapConn(&data, e.Conn)
	case infrared.PostConnProcessingEvent:
		mapProcessedConn(&data, e.ProcessedConn)
	case infrared.PreConnConnectingEvent:
		mapProcessedConn(&data, e.ProcessedConn)
		mapServer(&data, e.Server)
	case infrared.PlayerJoinEvent:
		mapProcessedConn(&data, e.ProcessedConn)
		mapServer(&data, e.Server)
	case infrared.PlayerLeaveEvent:
		mapProcessedConn(&data, e.ProcessedConn)
		mapServer(&data, e.Server)
	default:
		return
	}

	p.dispatchEvent(e, data)
}

func (p Plugin) dispatchEvent(e event.Event, data eventData) {
	var whks map[string][]webhook.Webhook
	switch data.edition {
	case infrared.JavaEdition:
		whks = p.javaWhks
	case infrared.BedrockEdition:
		whks = p.bedrockWhks
	default:
		p.log.Warn("failed to dispatch event")
	}

	el := webhook.EventLog{
		Topic:      e.Topic,
		OccurredAt: e.OccurredAt,
		Data:       data,
	}

	for _, wh := range whks[data.GatewayID] {
		if err := wh.DispatchEvent(el); err != nil && !errors.Is(err, webhook.ErrEventTypeNotAllowed) {
			p.log.Error("dispatching webhook event",
				zap.Error(err),
				zap.String("webhookId", wh.ID),
			)
		}
	}
}

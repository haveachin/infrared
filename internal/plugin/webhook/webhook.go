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

	logger   *zap.Logger
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
	p.logger = api.Logger()
	p.eventBus = api.EventBus()
	p.eventChs = map[uuid.UUID]event.Channel{}

	var err error
	p.javaWhks, err = p.loadWebhooks(infrared.JavaEdition)
	if err != nil {
		return err
	}
	p.bedrockWhks, err = p.loadWebhooks(infrared.BedrockEdition)
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
	Edition   string `json:"edition"`
	GatewayID string `json:"gatewayId"`
	Conn      struct {
		Network    string `json:"network"`
		LocalAddr  string `json:"localAddress"`
		RemoteAddr string `json:"remoteAddress"`
		Username   string `json:"username,omitempty"`
	} `json:"client"`
	Server struct {
		ServerID   string   `json:"serverId,omitempty"`
		ServerAddr string   `json:"serverAddress,omitempty"`
		Domains    []string `json:"domains,omitempty"`
	} `json:"server"`
	IsLoginRequest *bool `json:"isLoginRequest,omitempty"`
}

func mapConn(data *eventData, c infrared.Conn) {
	data.Edition = c.Edition().String()
	data.Conn.Network = c.LocalAddr().Network()
	data.Conn.LocalAddr = c.LocalAddr().String()
	data.Conn.RemoteAddr = c.RemoteAddr().String()
	data.GatewayID = c.GatewayID()
}

func mapProcessedConn(data *eventData, pc infrared.ProcessedConn) {
	mapConn(data, pc)
	data.Server.ServerAddr = pc.ServerAddr()
	data.Conn.Username = pc.Username()
	var isLoginRequest = pc.IsLoginRequest()
	data.IsLoginRequest = &isLoginRequest
}

func mapServer(data *eventData, s infrared.Server) {
	data.Server.ServerID = s.ID()
	data.Server.Domains = s.Domains()
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
	switch data.Edition {
	case infrared.JavaEdition.String():
		whks = p.javaWhks
	case infrared.BedrockEdition.String():
		whks = p.bedrockWhks
	default:
		p.logger.Warn("failed to dispatch event",
			zap.String("edition", data.Edition),
		)
	}

	el := webhook.EventLog{
		Topic:      e.Topic,
		OccurredAt: e.OccurredAt,
		Data:       data,
	}

	for _, wh := range whks[data.GatewayID] {
		if err := wh.DispatchEvent(el); err != nil && !errors.Is(err, webhook.ErrEventTypeNotAllowed) {
			p.logger.Error("dispatching webhook event",
				zap.Error(err),
				zap.String("webhookId", wh.ID),
			)
		}
	}
}

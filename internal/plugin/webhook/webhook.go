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
	Edition infrared.Edition

	logger   *zap.Logger
	eventBus event.Bus
	eventID  uuid.UUID
	// GatewayID mapped to webhooks
	whks map[string][]webhook.Webhook
}

func (p Plugin) Name() string {
	return "Webhook"
}

func (p Plugin) Version() string {
	return "internal"
}

func (p *Plugin) Load(v *viper.Viper) error {
	var err error
	p.whks, err = p.loadWebhooks(v)
	if err != nil {
		return err
	}
	return nil
}

func (p *Plugin) Reload(v *viper.Viper) error {
	return p.Load(v)
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.logger = api.Logger()
	p.eventBus = api.EventBus()

	id, _ := p.eventBus.AttachHandler(uuid.Nil, p.handleEvent)
	p.eventID = id

	return nil
}

func (p Plugin) Disable() error {
	p.eventBus.DetachRecipient(p.eventID)
	return nil
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

func unmarshalConn(data *eventData, c infrared.Conn) {
	data.Edition = c.Edition().String()
	data.Conn.Network = c.LocalAddr().Network()
	data.Conn.LocalAddr = c.LocalAddr().String()
	data.Conn.RemoteAddr = c.RemoteAddr().String()
	data.GatewayID = c.GatewayID()
}

func unmarshalProcessedConn(data *eventData, pc infrared.ProcessedConn) {
	unmarshalConn(data, pc)
	data.Server.ServerAddr = pc.ServerAddr()
	data.Conn.Username = pc.Username()
	var isLoginRequest = pc.IsLoginRequest()
	data.IsLoginRequest = &isLoginRequest
}

func unmarshalServer(data *eventData, s infrared.Server) {
	data.Server.ServerID = s.ID()
	data.Server.Domains = s.Domains()
}

func (p Plugin) handleEvent(e event.Event) {
	var data eventData
	switch e := e.Data.(type) {
	case infrared.NewConnEvent:
		unmarshalConn(&data, e.Conn)
	case infrared.PreConnProcessingEvent:
		unmarshalConn(&data, e.Conn)
	case infrared.PostConnProcessingEvent:
		unmarshalProcessedConn(&data, e.ProcessedConn)
	case infrared.PreConnConnectingEvent:
		unmarshalProcessedConn(&data, e.ProcessedConn)
		unmarshalServer(&data, e.Server)
	case infrared.PlayerJoinEvent:
		unmarshalProcessedConn(&data, e.ProcessedConn)
		unmarshalServer(&data, e.Server)
	case infrared.PlayerLeaveEvent:
		unmarshalProcessedConn(&data, e.ProcessedConn)
		unmarshalServer(&data, e.Server)
	default:
		return
	}

	p.dispatchEvent(e, data)
}

func (p Plugin) dispatchEvent(e event.Event, data eventData) {
	el := webhook.EventLog{
		Topics:     e.Topics,
		OccurredAt: e.OccurredAt,
		Data:       data,
	}

	for _, wh := range p.whks[data.GatewayID] {
		if err := wh.DispatchEvent(el); err != nil && !errors.Is(err, webhook.ErrEventTypeNotAllowed) {
			p.logger.Error("dispatching webhook event",
				zap.Error(err),
				zap.String("webhookId", wh.ID),
			)
		}
	}
}

package webhook

import (
	"errors"

	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/haveachin/infrared/pkg/webhook"
	"go.uber.org/zap"
)

type Plugin struct {
	Webhooks []webhook.Webhook

	log      *zap.Logger
	eventBus event.Bus
	eventChs map[uuid.UUID]event.Channel
	webhooks map[string]webhook.Webhook
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
	p.webhooks = map[string]webhook.Webhook{}
	for _, wh := range p.Webhooks {
		p.webhooks[wh.ID] = wh
	}

	for e := range ch {
		p.handleEvent(e)
	}
}

func mapConn(data map[string]interface{}, c infrared.Conn) {
	data["connNetwork"] = c.LocalAddr().Network()
	data["connLocalAddr"] = c.LocalAddr().String()
	data["connRemoteAddr"] = c.LocalAddr().String()
	data["gatewayId"] = c.LocalAddr().String()
}

func mapProcessedConn(data map[string]interface{}, pc infrared.ProcessedConn) {
	mapConn(data, pc)
	data["requestedServerAddr"] = pc.ServerAddr()
	data["username"] = pc.Username()
	data["isLoginRequest"] = pc.IsLoginRequest()
}

func mapServer(data map[string]interface{}, s infrared.Server) {
	data["serverId"] = s.ID()
}

func (p Plugin) handleEvent(e event.Event) {
	data := map[string]interface{}{}
	switch e := e.Data.(type) {
	case infrared.NewConnEvent:
		mapConn(data, e.Conn)
	case infrared.PreConnProcessingEvent:
		mapConn(data, e.Conn)
	case infrared.PostConnProcessingEvent:
		mapProcessedConn(data, e.ProcessedConn)
	case infrared.PreConnConnectingEvent:
		mapProcessedConn(data, e.ProcessedConn)
		mapServer(data, e.Server)
	case infrared.PlayerJoinEvent:
		mapProcessedConn(data, e.ProcessedConn)
		mapServer(data, e.Server)
	case infrared.PlayerLeaveEvent:
		mapProcessedConn(data, e.ProcessedConn)
		mapServer(data, e.Server)
	default:
		return
	}

	ids, ok := data["serverWebhookIds"].([]string)
	if !ok {
		return
	}

	el := webhook.EventLog{
		Type:       e.Topic,
		OccurredAt: e.OccurredAt,
		Data:       data,
	}

	for _, id := range ids {
		wh, ok := p.webhooks[id]
		if !ok {
			continue
		}

		if err := wh.DispatchEvent(el); err != nil && !errors.Is(err, webhook.ErrEventTypeNotAllowed) {
			p.log.Error("dispatching webhook event",
				zap.Error(err),
				zap.String("webhookId", id),
			)
		}
	}
}

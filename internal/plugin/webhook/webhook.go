package webhook

import (
	"errors"

	"github.com/go-logr/logr"
	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/haveachin/infrared/pkg/webhook"
)

type Plugin struct {
	Webhooks []webhook.Webhook

	log      logr.Logger
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
	for _, h := range p.Webhooks {
		p.webhooks[h.ID] = h
	}

	for e := range ch {
		p.handleEvent(e)
	}
}

func (p Plugin) handleEvent(e event.Event) {
	data, ok := parseMap(e.Data)
	if !ok {
		p.log.Info("failed processing event data",
			"eventTopic", e.Topic,
		)
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
			p.log.Error(err, "dispatching webhook event",
				"webhookId", id,
			)
		}
	}
}

func parseMap(v interface{}) (map[string]interface{}, bool) {
	kvs, ok := v.([]interface{})
	if !ok || len(kvs)%2 != 0 {
		return nil, false
	}

	data := map[string]interface{}{}
	for i := 0; i < len(kvs); i += 2 {
		k, ok := kvs[i].(string)
		if !ok {
			return nil, false
		}
		data[k] = kvs[i+1]
	}
	return data, true
}

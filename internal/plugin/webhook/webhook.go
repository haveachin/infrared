package webhook

import (
	"errors"

	"github.com/go-logr/logr"
	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/haveachin/infrared/pkg/webhook"
)

type WebhookPlugin struct {
	Webhooks []webhook.Webhook

	log logr.Logger
	eb  *event.Bus
	ec  map[uuid.UUID]event.Channel
}

func (p WebhookPlugin) Name() string {
	return "Webhook Plugin"
}

func (p WebhookPlugin) Version() string {
	return "internal"
}

func (p *WebhookPlugin) Enable(log logr.Logger, eb *event.Bus) error {
	p.log = log
	p.eb = eb
	p.ec = map[uuid.UUID]event.Channel{}

	c := make(event.Channel, 10)
	id, _ := eb.AttachChannel(uuid.Nil, c)
	p.ec[id] = c

	go p.start(c)

	return nil
}

func (p WebhookPlugin) Disable() error {
	for id, c := range p.ec {
		p.eb.DetachRecipient(id)
		close(c)
	}

	return nil
}

func (p WebhookPlugin) start(ch event.Channel) {
	whks := map[string]webhook.Webhook{}
	for _, h := range p.Webhooks {
		whks[h.ID] = h
	}

	for e := range ch {
		ids, ok := e.Data["serverWebhookIds"].([]string)
		if !ok {
			continue
		}

		el := webhook.EventLog{
			Type:       e.Topic,
			OccurredAt: e.CreatedAt,
			Data:       e.Data,
		}

		for _, id := range ids {
			h, ok := whks[id]
			if !ok {
				continue
			}

			if err := h.DispatchEvent(el); err != nil && !errors.Is(err, webhook.ErrEventTypeNotAllowed) {
				p.log.Error(err, "dispatching webhook event",
					"webhookId", h.ID,
				)
			}
		}
	}
}

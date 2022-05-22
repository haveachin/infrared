package webhook

import (
	"fmt"
	"net/http"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/webhook"
	"github.com/spf13/viper"
)

func (p Plugin) loadWebhooks(edition infrared.Edition) (map[string][]webhook.Webhook, error) {
	defaultsKey := fmt.Sprintf("defaults.%s.webhook", edition)
	key := fmt.Sprintf("%s.webhooks", edition)
	webhooks := map[string][]webhook.Webhook{}
	for id, v := range p.Viper.GetStringMap(key) {
		vpr := p.Viper.Sub(defaultsKey)
		if vpr == nil {
			vpr = viper.New()
		}
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg webhookConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		for _, gwID := range cfg.GatewayIDs {
			if webhooks[gwID] == nil {
				webhooks[gwID] = []webhook.Webhook{newWebhook(id, cfg)}
			} else {
				webhooks[gwID] = append(webhooks[gwID], newWebhook(id, cfg))
			}
		}
	}
	return webhooks, nil
}

type webhookConfig struct {
	DialTimeout time.Duration
	URL         string
	Events      []string
	GatewayIDs  []string
}

func newWebhook(id string, cfg webhookConfig) webhook.Webhook {
	return webhook.Webhook{
		ID: id,
		HTTPClient: &http.Client{
			Timeout: cfg.DialTimeout,
		},
		URL:           cfg.URL,
		AllowedTopics: cfg.Events,
	}
}

package webhook

import (
	"fmt"
	"net/http"
	"time"

	"github.com/haveachin/infrared/pkg/webhook"
	"github.com/spf13/viper"
)

func (p Plugin) loadWebhooks(v *viper.Viper) (map[string][]webhook.Webhook, error) {
	defaultsKey := fmt.Sprintf("defaults.%s.webhook", p.Edition)
	key := fmt.Sprintf("%s.webhooks", p.Edition)
	webhooks := map[string][]webhook.Webhook{}
	for id, m := range v.GetStringMap(key) {
		vpr := v.Sub(defaultsKey)
		if vpr == nil {
			vpr = viper.New()
		}
		vMap := m.(map[string]interface{})
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

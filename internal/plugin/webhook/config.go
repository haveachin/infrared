package webhook

import (
	"net/http"
	"time"

	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/pkg/webhook"
	"github.com/imdario/mergo"
)

func (p Plugin) loadWebhooks(c map[string]interface{}) (map[string][]webhook.Webhook, error) {
	var cfg PluginConfig
	if err := config.Unmarshal(c, &cfg); err != nil {
		return nil, err
	}

	webhooks := map[string][]webhook.Webhook{}
	for id, whCfg := range cfg.Webhooks {
		if err := mergo.Merge(&whCfg, cfg.Defaults.Webhook); err != nil {
			return nil, err
		}

		for _, gwID := range whCfg.GatewayIDs {
			if webhooks[gwID] == nil {
				webhooks[gwID] = []webhook.Webhook{newWebhook(id, whCfg)}
			} else {
				webhooks[gwID] = append(webhooks[gwID], newWebhook(id, whCfg))
			}
		}
	}
	return webhooks, nil
}

type PluginConfig struct {
	Webhooks map[string]webhookConfig `mapstructure:"webhooks"`
	Defaults struct {
		Webhook webhookConfig `mapstructure:"webhook"`
	} `mapstructure:"defaults"`
}

type webhookConfig struct {
	DialTimeout time.Duration `mapstructure:"dialTimeout"`
	URL         string        `mapstructure:"url"`
	Events      []string      `mapstructure:"events"`
	GatewayIDs  []string      `mapstructure:"gatewayIds"`
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

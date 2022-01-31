package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	_ "embed"

	"github.com/haveachin/infrared/pkg/webhook"
	"github.com/spf13/viper"
)

//go:embed config.default.yml
var defaultConfig []byte

func initConfig() {
	viper.SetConfigFile(configPath)
	if err := viper.ReadConfig(bytes.NewBuffer(defaultConfig)); err != nil {
		log.Fatal(err)
	}

	if err := createConfigIfNonExist(configPath); err != nil {
		log.Fatal(err)
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(err)
	}
}

type WebhookProxyConfig struct{}

func (cfg WebhookProxyConfig) LoadWebhooks() ([]webhook.Webhook, error) {
	vpr := viper.Sub("defaults.webhook")

	var webhooks []webhook.Webhook
	for id, v := range viper.GetStringMap("webhooks") {
		vMap := v.(map[string]interface{})
		if err := vpr.MergeConfigMap(vMap); err != nil {
			return nil, err
		}
		var cfg webhookConfig
		if err := vpr.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		webhooks = append(webhooks, newWebhook(id, cfg))
	}

	return webhooks, nil
}

func createConfigIfNonExist(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return ioutil.WriteFile(path, defaultConfig, 0644)
}

type webhookConfig struct {
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
	URL         string        `mapstructure:"url"`
	Events      []string      `mapstructure:"events"`
}

func newWebhook(id string, cfg webhookConfig) webhook.Webhook {
	return webhook.Webhook{
		ID: id,
		HTTPClient: &http.Client{
			Timeout: cfg.DialTimeout,
		},
		URL:        cfg.URL,
		EventTypes: cfg.Events,
	}
}

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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

func createConfigIfNonExist(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return ioutil.WriteFile(path, defaultConfig, 0644)
}

func LoadWebhooks() ([]webhook.Webhook, error) {
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
		URL:               cfg.URL,
		AllowedEventTypes: cfg.Events,
	}
}

func loadImageAndEncodeToBase64String(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	imgFile, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer imgFile.Close()

	bb, err := io.ReadAll(imgFile)
	if err != nil {
		return "", err
	}
	img64 := base64.StdEncoding.EncodeToString(bb)

	return fmt.Sprintf("data:image/png;base64,%s", img64), nil
}

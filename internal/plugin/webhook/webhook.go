package webhook

import (
	"errors"
	"net/http"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/haveachin/infrared/pkg/webhook"
	"github.com/imdario/mergo"
	"go.uber.org/zap"
)

type PluginConfig struct {
	Webhook struct {
		Enable   bool                     `mapstructure:"enable"`
		Webhooks map[string]webhookConfig `mapstructure:"webhooks"`
	} `mapstructure:"webhook"`
	Defaults struct {
		Webhook webhookConfig `mapstructure:"webhook"`
	} `mapstructure:"defaults"`
}

func (cfg PluginConfig) loadWebhooks() (map[string][]webhook.Webhook, error) {
	webhooks := map[string][]webhook.Webhook{}
	for id, whCfg := range cfg.Webhook.Webhooks {
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

type Plugin struct {
	Config   PluginConfig
	logger   *zap.Logger
	eventID  string
	// GatewayID mapped to webhooks
	whks map[string][]webhook.Webhook
}

func (p Plugin) Name() string {
	return "Webhook"
}

func (p Plugin) Version() string {
	return "internal"
}

func (p *Plugin) Init() {}

func (p *Plugin) Load(cfg map[string]any) error {
	pluginCfg := PluginConfig{}
	if err := config.Unmarshal(cfg, &pluginCfg); err != nil {
		return err
	}
	p.Config = pluginCfg

	if !p.Config.Webhook.Enable {
		return infrared.ErrPluginViaConfigDisabled
	}

	whks, err := p.Config.loadWebhooks()
	if err != nil {
		return err
	}
	p.whks = whks

	return nil
}

func (p *Plugin) Reload(cfg map[string]any) error {
	if err := p.Load(cfg); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.logger = api.Logger()

	p.eventID = p.eventBus.HandleFunc(p.handleEvent)

	return nil
}

func (p *Plugin) Disable() error {
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

func unmarshalProcessedConn(data *eventData, pc infrared.Player) {
	unmarshalConn(data, pc)
	data.Server.ServerAddr = pc.MatchedAddr()
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
	case infrared.AcceptedConnEvent:
		unmarshalConn(&data, e.Conn)
	case infrared.PreConnProcessingEvent:
		unmarshalConn(&data, e.Conn)
	case infrared.PostConnProcessingEvent:
		unmarshalProcessedConn(&data, e.Player)
	case infrared.PrePlayerJoinEvent:
		unmarshalProcessedConn(&data, e.Player)
		unmarshalServer(&data, e.Server)
	case infrared.PlayerJoinEvent:
		unmarshalProcessedConn(&data, e.Player)
		unmarshalServer(&data, e.Server)
	case infrared.PlayerLeaveEvent:
		unmarshalProcessedConn(&data, e.Player)
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

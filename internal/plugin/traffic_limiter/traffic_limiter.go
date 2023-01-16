package traffic_limiter

import (
	"errors"

	"github.com/c2h5oh/datasize"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type trafficLimiter struct {
	file                       string
	trafficLimit               datasize.ByteSize
	resetCron                  string
	storage                    storage
	OutOfBandwidthDisconnecter infrared.PlayerDisconnecter
}

type PluginConfig struct {
	TrafficLimiter struct {
		Enable          bool                            `mapstructure:"enable"`
		TrafficLimiters map[string]trafficLimiterConfig `mapstructure:"trafficLimiters"`
	} `mapstructure:"trafficLimiter"`
	Defaults struct {
		TrafficLimiter trafficLimiterConfig `mapstructure:"trafficLimiter"`
	} `mapstructure:"defaults"`
}

type Plugin struct {
	Config   PluginConfig
	logger   *zap.Logger
	eventBus event.Bus
	eventIDs []string
	// ServerID mapped to trafficLimiter
	trafficLimiters map[string]trafficLimiter
}

func (p Plugin) Name() string {
	return "Traffic Limiter"
}

func (p Plugin) Version() string {
	return "internal"
}

func (p *Plugin) Init() {}

func (p *Plugin) Load(cfg map[string]any) error {
	if err := config.Unmarshal(cfg, &p.Config); err != nil {
		return err
	}

	if !p.Config.TrafficLimiter.Enable {
		return infrared.ErrPluginViaConfigDisabled
	}

	trafficLimiters, err := p.Config.loadTrafficLimiterConfigs()
	if err != nil {
		return err
	}
	p.trafficLimiters = trafficLimiters

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
	p.eventBus = api.EventBus()

	p.registerEventHandler()
	p.startCronJobs()

	return nil
}

func (p Plugin) Disable() error {
	for _, id := range p.eventIDs {
		p.eventBus.DetachRecipient(id)
	}
	return nil
}

func (p *Plugin) startCronJobs() error {
	resetCron := cron.New()
	for srvID, tl := range p.trafficLimiters {
		if _, err := resetCron.AddJob(tl.resetCron, cron.FuncJob(func() {
			tl.storage.ResetConsumedBytes(srvID)
		})); err != nil {
			return err
		}
	}
	go resetCron.Start()
	return nil
}

func (p *Plugin) registerEventHandler() {
	p.eventIDs = append(p.eventIDs, p.eventBus.HandleFunc(p.onPreConnConnecting, infrared.PrePlayerJoinEventTopic))
	p.eventIDs = append(p.eventIDs, p.eventBus.HandleFuncAsync(p.onPlayerLeave, infrared.PlayerLeaveEventTopicAsync))
}

func (p Plugin) onPlayerLeave(e event.Event) {
	switch e := e.Data.(type) {
	case infrared.PlayerLeaveEvent:
		tl, ok := p.trafficLimiters[e.Server.ID()]
		if !ok {
			return
		}

		_, err := tl.storage.AddConsumedBytes(e.Server.ID(), e.ConsumedBytes)
		if err != nil {
			p.logger.Error("failed to add consumed bytes", zap.Error(err))
			return
		}
	}
}

func (p Plugin) onPreConnConnecting(e event.Event) (any, error) {
	switch e := e.Data.(type) {
	case infrared.PerPlayerJoinEvent:
		t, ok := p.trafficLimiters[e.Server.ID()]
		if !ok {
			return nil, nil
		}

		totalBytes, err := t.storage.ConsumedBytes(e.Server.ID())
		if err != nil {
			p.logger.Error("failed to read consumed bytes", zap.Error(err))
			return nil, nil
		}

		if t.trafficLimit <= datasize.ByteSize(totalBytes) {
			p.logger.Info("traffic limit reached", zap.String("serverID", e.Server.ID()))
			t.OutOfBandwidthDisconnecter.DisconnectPlayer(e.Player, infrared.ApplyTemplates(
				infrared.TimeMessageTemplates(),
				infrared.PlayerMessageTemplates(e.Player),
			))
			return nil, errors.New("traffic limit reached")
		}
	}
	return nil, nil
}

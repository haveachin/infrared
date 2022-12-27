package traffic_limiter

import (
	"errors"
	"fmt"

	"github.com/c2h5oh/datasize"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/haveachin/infrared/internal/pkg/java/protocol"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/login"
	"github.com/haveachin/infrared/internal/pkg/java/protocol/status"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type trafficLimiter struct {
	file                     string
	trafficLimit             datasize.ByteSize
	resetCron                string
	storage                  *storage
	OutOfBandwidthStatusJSON string
	OutOfBandwidthMessage    string
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

func (p Plugin) Init() {}

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
	case infrared.PreConnConnectingEvent:
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
			t.disconnectPlayer(e.ProcessedConn)
			return nil, errors.New("traffic limit reached")
		}
	}
	return nil, nil
}

func (t *trafficLimiter) disconnectPlayer(pc infrared.ProcessedConn) error {
	defer pc.Close()

	switch pc := pc.(type) {
	case *java.ProcessedConn:
		if pc.IsLoginRequest() {
			msg := infrared.ExecuteMessageTemplate(t.OutOfBandwidthMessage, pc)
			pk := login.ClientBoundDisconnect{
				Reason: protocol.Chat(fmt.Sprintf("{\"text\":\"%s\"}", msg)),
			}.Marshal()
			return pc.WritePacket(pk)
		}

		msg := infrared.ExecuteMessageTemplate(t.OutOfBandwidthStatusJSON, pc)
		pk := status.ClientBoundResponse{
			JSONResponse: protocol.String(msg),
		}.Marshal()

		if err := pc.WritePacket(pk); err != nil {
			return err
		}

		ping, err := pc.ReadPacket(status.MaxSizeServerBoundPingRequest)
		if err != nil {
			return err
		}

		return pc.WritePacket(ping)
	default:
		return errors.New("could not disconnect player")
	}
}

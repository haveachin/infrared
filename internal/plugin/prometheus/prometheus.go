package prometheus

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type PluginConfig struct {
	Prometheus struct {
		Enable bool   `mapstructure:"enable"`
		Bind   string `mapstructure:"bind"`
	} `mapstructure:"prometheus"`
}

type Plugin struct {
	Config   PluginConfig
	logger   *zap.Logger
	eventBus event.Bus
	eventID  string

	mux  http.Handler
	quit chan bool

	handshakeCount   *prometheus.CounterVec
	playersConnected *prometheus.GaugeVec
}

func (p Plugin) Name() string {
	return "Prometheus"
}

func (p Plugin) Version() string {
	return "internal"
}

func (p *Plugin) Init() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	p.mux = mux

	p.handshakeCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "infrared_handshakes",
		Help: "The total number of handshakes made to each Server per edition",
	}, []string{"host", "type", "edition"})
	p.playersConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infrared_connected",
		Help: "The total number of connected players per Server and edition",
	}, []string{"host", "server", "edition"})
}

func (p *Plugin) Load(cfg map[string]any) error {
	if err := config.Unmarshal(cfg, &p.Config); err != nil {
		return err
	}

	if !p.Config.Prometheus.Enable {
		return infrared.ErrPluginViaConfigDisabled
	}
	return nil
}

func (p *Plugin) Reload(cfg map[string]any) error {
	var pluginCfg PluginConfig
	if err := config.Unmarshal(cfg, &pluginCfg); err != nil {
		return err
	}

	if !pluginCfg.Prometheus.Enable {
		return infrared.ErrPluginViaConfigDisabled
	}

	if pluginCfg.Prometheus.Bind == p.Config.Prometheus.Bind {
		return nil
	}

	p.Config = pluginCfg
	p.quit <- true

	go p.startMetricsServer()
	return nil
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.logger = api.Logger()
	p.eventBus = api.EventBus()
	p.quit = make(chan bool)

	p.registerEventHandler()

	go p.startMetricsServer()
	return nil
}

func (p Plugin) Disable() error {
	p.eventBus.DetachRecipient(p.eventID)
	select {
	case p.quit <- true:
	default:
	}
	return nil
}

func (p *Plugin) registerEventHandler() {
	p.eventID = p.eventBus.HandleFuncAsync(p.handleEvent)
}

func (p Plugin) startMetricsServer() {
	srv := http.Server{
		Handler: p.mux,
		Addr:    p.Config.Prometheus.Bind,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, os.ErrClosed) {
			p.logger.Error("failed to start server", zap.Error(err))
			return
		}
	}()

	p.logger.Info("started prometheus metrics server",
		zap.String("bind", p.Config.Prometheus.Bind),
	)

	<-p.quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func (p Plugin) handleEvent(e event.Event) {
	switch e := e.Data.(type) {
	case infrared.PostConnProcessingEvent:
		edition := e.Player.Edition()
		domain := e.Player.ServerAddr()
		switch edition {
		case infrared.JavaEdition:
			if e.Player.IsLoginRequest() {
				p.handshakeCount.With(prometheus.Labels{"host": domain, "type": "login", "edition": "java"}).Inc()
			} else {
				p.handshakeCount.With(prometheus.Labels{"host": domain, "type": "status", "edition": "java"}).Inc()
			}
		case infrared.BedrockEdition:
			p.handshakeCount.With(prometheus.Labels{"host": domain, "type": "login", "edition": "bedrock"}).Inc()
		}
	case infrared.PlayerJoinEvent:
		edition := e.Player.Edition().String()
		server := e.Server.ID()
		domain := e.MatchedDomain
		p.playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition}).Inc()
	case infrared.PlayerLeaveEvent:
		edition := e.Player.Edition().String()
		server := e.Server.ID()
		domain := e.MatchedDomain
		p.playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition}).Dec()
		/*case infrared.ServerRegisterEvent:
		edition := e.Server.Edition().String()
		server := e.Server.ID()
		for _, domain := range e.Server.Domains() {
			domain = strings.ToLower(domain)
			p.playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition})
		}*/
	}
}

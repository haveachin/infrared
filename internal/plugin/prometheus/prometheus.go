package prometheus

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
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
	Config PluginConfig
	logger *zap.Logger

	eventChan chan any
	mux       http.Handler
	quit      chan bool

	handshakeCount   *prometheus.CounterVec
	playersConnected *prometheus.GaugeVec
	consumedBytes    *prometheus.CounterVec
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
		Help: "The total number of handshakes made to each server per edition",
	}, []string{"requested_domain", "request_type", "edition"})
	p.playersConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infrared_connected",
		Help: "The total number of connected players per server and edition",
	}, []string{"requested_domain", "matched_domain", "server_id", "edition"})
	p.consumedBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "infrared_consumed_bytes",
		Help: "The total number of consumed bytes made to each server per edition",
	}, []string{"server_id", "edition"})

	infrared.PostConnProcessingEvent.HandleFunc(p.handlePostConnProcessingEvent)
	infrared.PlayerJoinEvent.HandleFunc(p.handlePlayerJoinEvent)
	infrared.PlayerLeaveEvent.HandleFunc(p.handlePlayerLeaveEvent)
}

func (p *Plugin) Load(cfg map[string]any) error {
	pluginCfg := PluginConfig{}
	if err := config.Unmarshal(cfg, &pluginCfg); err != nil {
		return err
	}
	p.Config = pluginCfg

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
	p.eventChan = make(chan any, 100)
	p.quit = make(chan bool)

	go p.processEvents()
	go p.startMetricsServer()
	return nil
}

func (p *Plugin) Disable() error {
	close(p.eventChan)
	p.quit <- true
	close(p.quit)
	return nil
}

func (p Plugin) startMetricsServer() {
	srv := http.Server{
		Handler: p.mux,
		Addr:    p.Config.Prometheus.Bind,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

func (p Plugin) handlePostConnProcessingEvent(e infrared.PostConnProcessingPayload) bool {
	p.handleEvent(e)
	return true
}

func (p Plugin) handlePlayerJoinEvent(e infrared.PlayerJoinPayload) bool {
	p.handleEvent(e)
	return true
}

func (p Plugin) handlePlayerLeaveEvent(e infrared.PlayerLeavePayload) bool {
	p.handleEvent(e)
	return true
}

func (p Plugin) handleEvent(e any) {
	if !p.Config.Prometheus.Enable {
		return
	}

	select {
	case p.eventChan <- e:
	default:
		p.logger.Debug("rejected event; channel is full")
	}
}

func (p Plugin) processEvents() {
	for e := range p.eventChan {
		switch e := e.(type) {
		case infrared.PostConnProcessingPayload:
			requestType := "status"
			if e.Player.IsLoginRequest() {
				requestType = "login"
			}
			p.handshakeCount.With(prometheus.Labels{
				"requested_domain": e.Player.RequestedAddr(),
				"request_type":     requestType,
				"edition":          e.Player.Edition().String(),
			}).Inc()
		case infrared.PlayerJoinPayload:
			p.playersConnected.With(prometheus.Labels{
				"requested_domain": e.Player.RequestedAddr(),
				"matched_domain":   e.MatchedDomain,
				"server_id":        e.Server.ID(),
				"edition":          e.Player.Edition().String(),
			}).Inc()
		case infrared.PlayerLeavePayload:
			p.playersConnected.With(prometheus.Labels{
				"requested_domain": e.Player.RequestedAddr(),
				"matched_domain":   e.MatchedDomain,
				"server_id":        e.Server.ID(),
				"edition":          e.Player.Edition().String(),
			}).Dec()
			p.consumedBytes.With(prometheus.Labels{
				"server_id": e.Server.ID(),
				"edition":   e.Player.Edition().String(),
			}).Add(float64(e.ConsumedBytes))
		}
	}
}

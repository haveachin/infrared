package prometheus

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Plugin struct {
	logger   *zap.Logger
	eventBus event.Bus
	eventID  uuid.UUID
	bind     string
	quit     chan bool

	handshakeCount   *prometheus.CounterVec
	playersConnected *prometheus.GaugeVec
}

func (p Plugin) Name() string {
	return "Prometheus"
}

func (p Plugin) Version() string {
	return fmt.Sprint("internal")
}

func (p *Plugin) Load(v *viper.Viper) error {
	p.bind = v.GetString("prometheus.bind")
	if p.bind == "" {
		return errors.New("prometheus bind empty, not enabling prometheus plugin")
	}

	p.handshakeCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "infrared_handshakes",
		Help: "The total number of handshakes made to each Server per edition",
	}, []string{"host", "type", "edition"})
	p.playersConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infrared_connected",
		Help: "The total number of connected players per Server and edition",
	}, []string{"host", "server", "edition"})

	return nil
}

func (p *Plugin) Reload(v *viper.Viper) error {
	bind := v.GetString("prometheus.bind")
	if p.bind == bind {
		return nil
	}

	if bind == "" {
		return p.Disable()
	}

	if err := p.Disable(); err != nil {
		return err
	}

	go p.startMetricsServer()
	return nil
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.logger = api.Logger()
	p.eventBus = api.EventBus()

	id, _ := p.eventBus.AttachHandler(uuid.Nil, p.handleEvent)
	p.eventID = id
	p.quit = make(chan bool, 1)

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

func (p Plugin) startMetricsServer() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := http.Server{
		Handler: mux,
		Addr:    p.bind,
	}

	if err := srv.ListenAndServe(); err != nil {
		p.logger.Error("failed to start server", zap.Error(err))
		return
	}
	<-p.quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func (p Plugin) handleEvent(e event.Event) {
	switch e := e.Data.(type) {
	case infrared.PostConnProcessingEvent:
		edition := e.ProcessedConn.Edition()
		domain := e.ProcessedConn.ServerAddr()
		switch edition {
		case infrared.JavaEdition:
			if e.ProcessedConn.IsLoginRequest() {
				p.handshakeCount.With(prometheus.Labels{"host": domain, "type": "login", "edition": "java"}).Inc()
			} else {
				p.handshakeCount.With(prometheus.Labels{"host": domain, "type": "status", "edition": "java"}).Inc()
			}
		case infrared.BedrockEdition:
			p.handshakeCount.With(prometheus.Labels{"host": domain, "type": "login", "edition": "bedrock"}).Inc()
		}
	case infrared.PlayerJoinEvent:
		edition := e.ProcessedConn.Edition().String()
		server := e.Server.ID()
		domain := e.MatchedDomain
		p.playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition}).Inc()
	case infrared.PlayerLeaveEvent:
		edition := e.ProcessedConn.Edition().String()
		server := e.Server.ID()
		domain := e.MatchedDomain
		p.playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition}).Dec()
	case infrared.ServerRegisterEvent:
		edition := e.Server.Edition().String()
		server := e.Server.ID()
		for _, domain := range e.Server.Domains() {
			domain = strings.ToLower(domain)
			p.playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition})
		}
	}
}

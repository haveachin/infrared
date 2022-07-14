package prometheus

import (
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/pkg/event"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Plugin struct {
	logger   *zap.Logger
	eventBus event.Bus
	eventID  uuid.UUID
	promBind string
}

var (
	handshakeCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "infrared_handshakes",
		Help: "The total number of handshakes made to each Server per edition",
	}, []string{"host", "type", "edition"})
	playersConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "infrared_connected",
		Help: "The total number of connected players per Server and edition",
	}, []string{"host", "server", "edition"})
)

func (p Plugin) Name() string {
	return "Prometheus"
}

func (p Plugin) Version() string {
	return fmt.Sprint("internal")
}

func (p *Plugin) Load(v *viper.Viper) error {
	p.promBind = v.GetString("prometheus.bind")
	if p.promBind == "" {
		return errors.New("prometheus bind empty, not enabling prometheus plugin")
	}
	return nil
}

func (p *Plugin) Reload(v *viper.Viper) error {
	return p.Load(v)
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.logger = api.Logger()
	p.eventBus = api.EventBus()

	id, _ := p.eventBus.AttachHandler(uuid.Nil, p.handleEvent)
	p.eventID = id

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		p.logger.Info("starting prometheus listener", zap.String("address", p.promBind))

		err := http.ListenAndServe(p.promBind, nil)
		if err != nil {
			return
		}
	}()
	return nil
}

func (p Plugin) Disable() error {
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

func unmarshalProcessedConn(data *eventData, pc infrared.ProcessedConn) {
	unmarshalConn(data, pc)
	data.Server.ServerAddr = pc.ServerAddr()
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
	case infrared.PostConnProcessingEvent:
		unmarshalProcessedConn(&data, e.ProcessedConn)
	case infrared.PlayerJoinEvent:
		unmarshalProcessedConn(&data, e.ProcessedConn)
		unmarshalServer(&data, e.Server)
	case infrared.PlayerLeaveEvent:
		unmarshalProcessedConn(&data, e.ProcessedConn)
		unmarshalServer(&data, e.Server)
	case infrared.ServerRegisterEvent:
		unmarshalServer(&data, e.Server)
	default:
		return
	}

	p.logEvent(e, data)
}

func (p Plugin) logEvent(e event.Event, data eventData) {
	switch e := e.Data.(type) {
	case infrared.PostConnProcessingEvent:
		edition := e.ProcessedConn.Edition()
		domain := e.ProcessedConn.ServerAddr()
		if edition == infrared.JavaEdition {
			if e.ProcessedConn.IsLoginRequest() {
				handshakeCount.With(prometheus.Labels{"host": domain, "type": "login", "edition": "java"}).Inc()
			} else {
				handshakeCount.With(prometheus.Labels{"host": domain, "type": "status", "edition": "java"}).Inc()
			}
		} else {
			handshakeCount.With(prometheus.Labels{"host": domain, "type": "login", "edition": "bedrock"}).Inc()
		}
	case infrared.PlayerJoinEvent:
		edition := e.ProcessedConn.Edition().String()
		server := e.Server.ID()
		domain := e.Server.Domains()[0]
		playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition}).Inc()
	case infrared.PlayerLeaveEvent:
		edition := e.ProcessedConn.Edition().String()
		server := e.Server.ID()
		domain := e.Server.Domains()[0]
		playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition}).Dec()
	case infrared.ServerRegisterEvent:
		edition := e.Server.Edition().String()
		server := e.Server.ID()
		domain := e.Server.Domains()[0]
		playersConnected.With(prometheus.Labels{"host": domain, "server": server, "edition": edition})
	}
}

package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gofrs/uuid"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/pkg/event"
	"go.uber.org/zap"
)

type PluginConfig struct {
	API struct {
		Bind           string   `mapstructure:"bind"`
		AllowedOrigins []string `mapstructure:"allowedOrigins"`
	} `mapstructure:"api"`
}

type Plugin struct {
	Config   PluginConfig
	logger   *zap.Logger
	eventBus event.Bus
	eventID  uuid.UUID

	mux  http.Handler
	quit chan bool
}

func (p Plugin) Name() string {
	return "API"
}

func (p Plugin) Version() string {
	return "internal"
}

func (p *Plugin) Load(cfg map[string]interface{}) error {
	if err := config.Unmarshal(cfg, &p.Config); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) Reload(cfg map[string]interface{}) error {
	var pluginCfg PluginConfig
	if err := config.Unmarshal(cfg, &pluginCfg); err != nil {
		return err
	}

	if pluginCfg.API.Bind == p.Config.API.Bind {
		return nil
	}

	if pluginCfg.API.Bind == "" {
		return p.Disable()
	}

	p.Config = pluginCfg
	p.quit <- true

	go p.startAPIServer()
	return nil
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	if p.Config.API.Bind == "" {
		return nil
	}

	p.logger = api.Logger()
	p.eventBus = api.EventBus()
	p.quit = make(chan bool)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   p.Config.API.AllowedOrigins,
		AllowedMethods:   []string{"GET", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
	}))
	r.Route("/{edition}/players", func(r chi.Router) {
		r.Get("/{username}", getPlayerHandler(api))
		r.Get("/", getPlayersHandler(api))
		r.Delete("/{username}", deletePlayerHandler(api))
	})
	p.mux = r

	go p.startAPIServer()
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

func (p Plugin) startAPIServer() {
	srv := http.Server{
		Handler: p.mux,
		Addr:    p.Config.API.Bind,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			p.logger.Error("failed to start server", zap.Error(err))
			return
		}
	}()

	p.logger.Info("started api server",
		zap.String("bind", p.Config.API.Bind),
	)

	<-p.quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

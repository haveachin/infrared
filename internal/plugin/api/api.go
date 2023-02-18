package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	embed_api "github.com/haveachin/infrared/api"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"go.uber.org/zap"
)

type PluginConfig struct {
	API struct {
		Enable         bool     `mapstructure:"enable"`
		Bind           string   `mapstructure:"bind"`
		AllowedOrigins []string `mapstructure:"allowedOrigins"`
		AllowedMethods []string `mapstructure:"allowedMethods"`
		AllowedHeaders []string `mapstructure:"allowedHeaders"`
	} `mapstructure:"api"`
}

type Plugin struct {
	Config PluginConfig
	logger *zap.Logger
	api    infrared.API
	cfgAPI config.Config

	quit chan bool
}

func (p Plugin) Name() string {
	return "API"
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

	if !p.Config.API.Enable {
		return infrared.ErrPluginViaConfigDisabled
	}

	return nil
}

func (p *Plugin) Reload(cfg map[string]any) error {
	var pluginCfg PluginConfig
	if err := config.Unmarshal(cfg, &pluginCfg); err != nil {
		return err
	}

	if !pluginCfg.API.Enable {
		return infrared.ErrPluginViaConfigDisabled
	}

	if pluginCfg.API.Bind == p.Config.API.Bind {
		return nil
	}

	p.Config = pluginCfg
	p.quit <- true

	go p.startAPIServer()
	return nil
}

func (p *Plugin) Enable(api infrared.PluginAPI) error {
	p.logger = api.Logger()
	p.api = api
	p.cfgAPI = api.Config()
	p.quit = make(chan bool)

	go p.startAPIServer()
	return nil
}

func (p Plugin) Disable() error {
	select {
	case p.quit <- true:
	default:
	}
	return nil
}

func (p Plugin) startAPIServer() {
	srv := http.Server{
		Handler: p.router(),
		Addr:    p.Config.API.Bind,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

//	@title			Infrared API
//	@version		1.0
//	@description	API that is baked into Infrared via internal plugin.

//	@license.name	AGPL-3.0
//	@license.url	https://www.gnu.org/licenses/agpl-3.0.txt

//	@BasePath	/v1
func (p Plugin) router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   p.Config.API.AllowedOrigins,
		AllowedMethods:   p.Config.API.AllowedMethods,
		AllowedHeaders:   p.Config.API.AllowedHeaders,
		AllowCredentials: false,
	}))

	openAPIPath := "/swagger.yaml"
	r.HandleFunc(openAPIPath, func(w http.ResponseWriter, r *http.Request) {
		w.Write(embed_api.SwaggerSpecs)
	})
	r.Handle("/docs", swaggerUIHandler(SwaggerUIOpts{
		SpecURL: openAPIPath,
		Title:   "Infrared API Docs",
	}))

	r.Route("/v1", func(r chi.Router) {
		r.Route("/{edition}/players", func(r chi.Router) {
			r.Get("/{username}", getPlayerHandler(p.api))
			r.Get("/", getPlayersHandler(p.api))
			r.Delete("/{username}", deletePlayerHandler(p.api))
		})

		r.Route("/configs", func(r chi.Router) {
			r.Get("/{configID}", getConfig(p.cfgAPI))
			r.Get("/", getConfigs(p.cfgAPI))
			r.Post("/reload", reloadConfigs(p.cfgAPI))
			r.Put("/{configID}", putConfig())
			r.Delete("/{configID}", deleteConfig())
		})
	})
	return r
}

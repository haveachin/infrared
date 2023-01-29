package api

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/gorilla/schema"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/internal/pkg/config/provider"
)

type errorDTO struct {
	Message string `json:"message"`
}

func newErrorDTO(err error) errorDTO {
	return errorDTO{
		Message: err.Error(),
	}
}

func getPlayerHandler(api infrared.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		editionString := chi.URLParam(r, "edition")
		edition, err := infrared.EditionFromString(editionString)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		username := chi.URLParam(r, "username")
		player := api.PlayerByUsername(username, edition)
		if player == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		dto := struct {
			Username      string `json:"username"`
			GatewayID     string `json:"gatewayId"`
			RemoteAddr    string `json:"remoteAddress"`
			LocalAddr     string `json:"localAddress"`
			Version       string `json:"version"`
			ServerID      string `json:"serverId"`
			MatchedAddr   string `json:"matchedAddress"`
			RequestedAddr string `json:"requestedAddress"`
		}{
			Username:      player.Username(),
			GatewayID:     player.GatewayID(),
			RemoteAddr:    player.RemoteAddr().String(),
			LocalAddr:     player.LocalAddr().String(),
			Version:       player.Version().Name(),
			ServerID:      player.ServerID(),
			MatchedAddr:   player.MatchedAddr(),
			RequestedAddr: player.RequestedAddr(),
		}

		render.JSON(w, r, dto)
	}
}

func getPlayersHandler(api infrared.API) http.HandlerFunc {
	decoder := schema.NewDecoder()
	return func(w http.ResponseWriter, r *http.Request) {
		reqDTO := &struct {
			UsernameRegex string `schema:"usernameRegex"`
		}{}

		if err := decoder.Decode(reqDTO, r.URL.Query()); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		editionString := chi.URLParam(r, "edition")
		edition, err := infrared.EditionFromString(editionString)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		players, err := api.Players(reqDTO.UsernameRegex, edition)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		type respDTO struct {
			Username   string `json:"username"`
			RemoteAddr string `json:"remoteAddress"`
			GatewayID  string `json:"gatewayId"`
			ServerID   string `json:"serverId"`
		}

		respDTOs := make([]respDTO, len(players))
		for i, p := range players {
			respDTOs[i] = respDTO{
				Username:   p.Username(),
				RemoteAddr: p.RemoteAddr().String(),
				GatewayID:  p.GatewayID(),
				ServerID:   p.ServerID(),
			}
		}

		render.JSON(w, r, respDTOs)
	}
}

func deletePlayerHandler(api infrared.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		editionString := chi.URLParam(r, "edition")
		edition, err := infrared.EditionFromString(editionString)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		username := chi.URLParam(r, "username")
		player := api.PlayerByUsername(username, edition)
		if player == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		player.Close()

		w.WriteHeader(http.StatusNoContent)
	}
}

func getConfig(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configID := chi.URLParam(r, "configID")
		configPath, err := url.PathUnescape(configID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		cfg := map[string]any{}
		if err := provider.ReadConfigFile(configPath, cfg); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		render.JSON(w, r, cfg)
	}
}

func getConfigs(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prov := cfg.Providers()
		fileProv := prov[provider.FileType].(*provider.File)
		dockerProv := prov[provider.DockerType].(*provider.Docker)
		dockerCfg, err := dockerProv.Config()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		dto := struct {
			File   map[string]map[string]any `json:"file,omitempty"`
			Docker map[string]any            `json:"docker,omitempty"`
		}{
			File:   fileProv.Configs(),
			Docker: dockerCfg,
		}

		render.JSON(w, r, dto)
	}
}

func putConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configID := chi.URLParam(r, "configID")
		configPath, err := url.PathUnescape(configID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		cfg := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		if err := provider.WriteConfigFile(configPath, cfg); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		w.WriteHeader(http.StatusCreated)
		render.JSON(w, r, cfg)
	}
}

func deleteConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configID := chi.URLParam(r, "configID")
		configPath, err := url.PathUnescape(configID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		if err := provider.RemoveConfigFile(configPath); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

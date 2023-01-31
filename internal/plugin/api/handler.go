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
} //	@name	Error

func newErrorDTO(err error) errorDTO {
	return errorDTO{
		Message: err.Error(),
	}
}

//	@Summary		Get a player
//	@Description	Get a player for an edition by username
//	@Tags			Player
//	@Produce		json
//	@Param			edition		path		string	true	"Minecraft edition"	Enums(java, bedrock)
//	@Param			username	path		string	true	"Player username"
//	@Success		200			{object}	api.getPlayerHandler.respDTO
//	@Failure		400			{object}	api.errorDTO
//	@Failure		404
//	@Router			/{edition}/players/{username} [get]
func getPlayerHandler(api infrared.API) http.HandlerFunc {
	type respDTO struct {
		Username      string `json:"username" example:"H4v34ch1n"`
		GatewayID     string `json:"gatewayId" example:"default"`
		RemoteAddr    string `json:"remoteAddress" example:"123.45.67.89:45372"`
		LocalAddr     string `json:"localAddress" example:"127.0.0.1:54265"`
		Version       string `json:"version" example:"1.19.3"`
		ServerID      string `json:"serverId" example:"default"`
		MatchedAddr   string `json:"matchedAddress" example:"*.example.com"`
		RequestedAddr string `json:"requestedAddress" example:"mc.example.com"`
	} //	@Name	Player

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

		dto := respDTO{
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

//	@Summary		Query players per edition
//	@Description	Query players per edition and filter them by username via regular expression
//	@Tags			Player
//	@Produce		json
//	@Param			edition			path		string	true	"Minecraft edition"	Enums(java, bedrock)
//	@Param			usernameRegex	query		string	false	"A regular expression to query usernames"
//	@Success		200				{array}		api.getPlayersHandler.respDTOItem
//	@Failure		400				{object}	api.errorDTO
//	@Failure		422				{object}	api.errorDTO
//	@Router			/{edition}/players [get]
func getPlayersHandler(api infrared.API) http.HandlerFunc {
	type reqDTOQuery struct {
		UsernameRegex string `schema:"usernameRegex"`
	}

	type respDTOItem struct {
		Username   string `json:"username" example:"H4v34ch1n"`
		RemoteAddr string `json:"remoteAddress" example:"123.45.67.89:45372"`
		GatewayID  string `json:"gatewayId" example:"default"`
		ServerID   string `json:"serverId" example:"default"`
	} //	@Name	PlayerItem

	decoder := schema.NewDecoder()
	return func(w http.ResponseWriter, r *http.Request) {
		reqDTOQuery := &reqDTOQuery{}
		if err := decoder.Decode(reqDTOQuery, r.URL.Query()); err != nil {
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

		players, err := api.Players(reqDTOQuery.UsernameRegex, edition)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		respDTO := make([]respDTOItem, len(players))
		for i, p := range players {
			respDTO[i] = respDTOItem{
				Username:   p.Username(),
				RemoteAddr: p.RemoteAddr().String(),
				GatewayID:  p.GatewayID(),
				ServerID:   p.ServerID(),
			}
		}

		render.JSON(w, r, respDTO)
	}
}

//	@Summary		Disconnect a player
//	@Description	Disconnect a player by edition via username
//	@Tags			Player
//	@Produce		json
//	@Param			edition		path	string	true	"Minecraft edition"	Enums(java, bedrock)
//	@Param			username	path	string	true	"Player username"
//	@Success		204
//	@Failure		400	{object}	api.errorDTO
//	@Failure		404
//	@Router			/{edition}/players/{username} [delete]
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

//	@Summary		Get a config
//	@Description	Get a config via ID
//	@Tags			Config
//	@Produce		json
//	@Param			configId	path		string	true	"Config ID"
//	@Success		200			{string}	string	"See the documentation or configs folder for more info on this complex struct"
//	@Failure		400			{object}	api.errorDTO
//	@Failure		500			{object}	api.errorDTO
//	@Router			/configs/{configId} [get]
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

//	@Summary		Get all configs
//	@Description	Get all configs from all providers
//	@Tags			Config
//	@Produce		json
//	@Success		200	{object}	api.getConfigs.respDTO
//	@Failure		500	{object}	api.errorDTO
//	@Router			/configs [get]
func getConfigs(cfg config.Config) http.HandlerFunc {
	type respDTO struct {
		File   map[string]map[string]any `json:"file,omitempty"`
		Docker map[string]any            `json:"docker,omitempty"`
	} //	@Name	Configs

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

		respDTO := respDTO{
			File:   fileProv.Configs(),
			Docker: dockerCfg,
		}

		render.JSON(w, r, respDTO)
	}
}

//	@Summary		Create/Update a config
//	@Description	Create/Update a config via ID
//	@Tags			Config
//	@Produce		json
//	@Param			configId	path		string	true	"Config ID"
//	@Success		201			{string}	string	"See the documentation or configs folder for more info on this complex struct"
//	@Failure		400			{object}	api.errorDTO
//	@Failure		500			{object}	api.errorDTO
//	@Router			/configs/{configId} [put]
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

//	@Summary		Delete a config
//	@Description	Delete a config via ID
//	@Tags			Config
//	@Produce		json
//	@Param			configId	path	string	true	"Config ID"
//	@Success		204
//	@Failure		400	{object}	api.errorDTO
//	@Failure		500	{object}	api.errorDTO
//	@Router			/configs/{configId} [delete]
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

//	@Summary		Reloads Infrared
//	@Description	Reads all configs and reloads Infrared
//	@Tags			Config
//	@Produce		json
//	@Success		200	{string}	string	"See the documentation or configs folder for more info on this complex struct"
//	@Failure		500	{object}	api.errorDTO
//	@Router			/configs/reload [post]
func reloadConfigs(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := cfg.Reload()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		render.JSON(w, r, cfg)
	}
}

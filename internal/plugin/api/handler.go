package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/gorilla/schema"
	"github.com/haveachin/infrared/internal/app/infrared"
)

func getPlayerHandler(api infrared.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		editionString := chi.URLParam(r, "edition")
		edition, err := infrared.EditionFromString(editionString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
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
			ServerAddr    string `json:"serverAddress"`
			RequestedAddr string `json:"requestedAddress"`
		}{
			Username:      player.Username(),
			GatewayID:     player.GatewayID(),
			RemoteAddr:    player.RemoteAddr().String(),
			LocalAddr:     player.LocalAddr().String(),
			Version:       player.Version().Name(),
			ServerID:      player.ServerID(),
			ServerAddr:    player.MatchedAddr(),
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
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}

		editionString := chi.URLParam(r, "edition")
		edition, err := infrared.EditionFromString(editionString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}

		players, err := api.Players(reqDTO.UsernameRegex, edition)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
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

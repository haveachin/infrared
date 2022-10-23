package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/gorilla/schema"
	"github.com/haveachin/infrared/internal/app/infrared"
)

type errorDTO struct {
	Message string `json:"message"`
}

func newErrorDTO(err error) errorDTO {
	return errorDTO{
		Message: err.Error(),
	}
}

type getPlayersRequestDTO struct {
	UsernameRegex string `schema:"usernameRegex"`
}

type playerDTO struct {
	Username   string `json:"username"`
	GatewayID  string `json:"gatewayId"`
	RemoteAddr string `json:"remoteAddress"`
	LocalAddr  string `json:"localAddress"`
}

func newPlayerDTO(p infrared.Player) playerDTO {
	return playerDTO{
		Username:   p.Username(),
		GatewayID:  p.GatewayID(),
		RemoteAddr: p.RemoteAddr().String(),
		LocalAddr:  p.LocalAddr().String(),
	}
}

func getPlayerHandler(api infrared.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		editionString := chi.URLParam(r, "edition")
		edition, err := infrared.EditionFromString(editionString)
		if err != nil {
			render.Status(r, http.StatusUnprocessableEntity)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		username := chi.URLParam(r, "username")
		player := api.PlayerByUsername(edition, username)
		if player == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		render.JSON(w, r, newPlayerDTO(player))
	}
}

func getPlayersHandler(api infrared.API) http.HandlerFunc {
	decoder := schema.NewDecoder()
	return func(w http.ResponseWriter, r *http.Request) {
		var requestDTO getPlayersRequestDTO
		if err := decoder.Decode(&requestDTO, r.URL.Query()); err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		editionString := chi.URLParam(r, "edition")
		edition, err := infrared.EditionFromString(editionString)
		if err != nil {
			render.Status(r, http.StatusUnprocessableEntity)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		players, err := api.Players(edition, requestDTO.UsernameRegex)
		if err != nil {
			render.Status(r, http.StatusUnprocessableEntity)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		playerDTOs := make([]playerDTO, len(players))
		for i, p := range players {
			playerDTOs[i] = newPlayerDTO(p)
		}

		render.JSON(w, r, playerDTOs)
	}
}

func deletePlayerHandler(api infrared.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		editionString := chi.URLParam(r, "edition")
		edition, err := infrared.EditionFromString(editionString)
		if err != nil {
			render.Status(r, http.StatusUnprocessableEntity)
			render.JSON(w, r, newErrorDTO(err))
			return
		}

		username := chi.URLParam(r, "username")
		player := api.PlayerByUsername(edition, username)
		if player == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		player.Close()
		w.WriteHeader(http.StatusNoContent)
	}
}

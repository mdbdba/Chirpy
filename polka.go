package main

import (
	"encoding/json"
	"net/http"

	"github.com/Chirpy/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlePolka(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Event string            `json:"event"`
		Data  map[string]string `json:"data"`
	}

	w.Header().Set("Content-Type", "application/json")
	if r.Body == nil {
		respondWithError(w, http.StatusBadRequest, "No body")
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}
	if params.Event != "user.upgraded" {
		respondWithError(w, http.StatusNoContent, "Event not supported")
		return
	}
	parsedUserId, err := uuid.Parse(params.Data["user_id"])
	if err != nil {
		return
	}
	requestApiKey, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if requestApiKey != cfg.apiToken {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	_, err = cfg.dbQueries.UpgradeUserById(r.Context(), parsedUserId)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	respondWithJSON(w, http.StatusNoContent, "User upgraded")

}

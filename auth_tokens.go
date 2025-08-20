package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Chirpy/internal/auth"
	"github.com/Chirpy/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handleRevoke(w http.ResponseWriter, r *http.Request) {
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
	}
	token, err := cfg.dbQueries.GetRefreshTokenByToken(r.Context(), bearerToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = cfg.dbQueries.RevokeRefreshToken(r.Context(), token.Token)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusNoContent, "")

}

func (cfg *apiConfig) mkJWT(userID uuid.UUID, expiresIn time.Duration) (string, error) {
	return auth.MakeJWT(userID, cfg.svrToken, expiresIn)
}

func (cfg *apiConfig) handleRefresh(w http.ResponseWriter, r *http.Request) {

	type refreshResponse struct {
		Token string `json:"token"`
	}
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, err := cfg.dbQueries.GetRefreshTokenByToken(r.Context(), bearerToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if token.ExpiresAt.Before(time.Now()) {
		respondWithError(w, http.StatusUnauthorized, "Token expired")
		return
	}
	if token.RevokedAt.Valid && !token.RevokedAt.Time.IsZero() {
		respondWithError(w, http.StatusUnauthorized, "Token revoked")
		return
	}

	jwt, err := cfg.mkJWT(token.UserID, time.Duration(60*60)*time.Second)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if r.Body == nil {
		respondWithError(w, http.StatusBadRequest, "No body")
	}

	respondWithJSON(w, http.StatusOK, refreshResponse{Token: jwt})
}

func (cfg *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		// ExpiresInSeconds int    `json:"expires_in_seconds,omitempty"`
	}
	JWTExpiresInSeconds := 60 * 60                           // 1 hour
	RefreshTokenDefaultExpiresInSeconds := 60 * 60 * 24 * 60 // 60 days
	w.Header().Set("Content-Type", "application/json")
	if r.Body == nil {
		respondWithError(w, http.StatusBadRequest, "No body")
		return
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	user, err := cfg.dbQueries.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}
	err = auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	jwt, err := cfg.mkJWT(user.ID, time.Duration(JWTExpiresInSeconds)*time.Second)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "JWT creation error")
		return
	}
	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Refresh token creation error")
		return
	}
	// Add refresh token to database
	crParams := database.CreateRefreshTokenParams{Token: refreshToken, UserID: user.ID, ExpiresAt: time.Now().Add(time.Duration(RefreshTokenDefaultExpiresInSeconds) * time.Second)}
	refresh, err := cfg.dbQueries.CreateRefreshToken(r.Context(), crParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := userResponse{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt.String(),
		UpdatedAt:    user.UpdatedAt.String(),
		Email:        user.Email,
		Token:        jwt,
		RefreshToken: refresh.Token,
		IsChirpyRed:  user.IsChirpyRed,
	}
	respondWithJSON(w, http.StatusOK, response)

}

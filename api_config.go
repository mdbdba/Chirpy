package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/Chirpy/internal/auth"
	"github.com/Chirpy/internal/database"
	"github.com/google/uuid"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
	svrToken       string
	apiToken       string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			cfg.fileserverHits.Add(1)
			next.ServeHTTP(w, r)
		},
	)
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(fmt.Sprintf("<html>\n  <body>\n    <h1>Welcome, Chirpy Admin</h1>\n    <p>Chirpy has been visited %d times!</p>\n  </body>\n</html>", cfg.fileserverHits.Load())))
	if err != nil {
		return
	}
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	err := cfg.dbQueries.DeleteUsers(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, "Reset")
}

func (cfg *apiConfig) handleUsers(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	w.Header().Set("Content-Type", "application/json")
	if r.Body == nil {
		respondWithError(w, http.StatusBadRequest, "No body")
		return
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
	}

	password, err := auth.HashPassword(params.Password)
	if err != nil {
		return
	}

	createUser, err := cfg.dbQueries.CreateUser(r.Context(),
		database.CreateUserParams{Email: params.Email, HashedPassword: password})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusCreated,
		user{ID: createUser.ID, CreatedAt: createUser.CreatedAt.String(),
			UpdatedAt:   createUser.UpdatedAt.String(),
			Email:       createUser.Email,
			IsChirpyRed: createUser.IsChirpyRed,
		})
}

func (cfg *apiConfig) handleGetChirps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Check for author_id query parameter
	authorIDParam := r.URL.Query().Get("author_id")
	if authorIDParam != "" {
		// Handle filtering by author ID
		parsedAuthorID, err := uuid.Parse(authorIDParam)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid author ID format")
			return
		}

		chirps, err := cfg.dbQueries.GetChirpsByUserId(r.Context(), parsedAuthorID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		returnChirps := make([]chirp, len(chirps))
		for i, v := range chirps {
			returnChirps[i] = chirp{
				ID:        v.ID,
				CreatedAt: v.CreatedAt.String(),
				UpdatedAt: v.UpdatedAt.String(),
				Body:      v.Body,
				UserID:    v.UserID,
			}
		}
		respondWithJSON(w, http.StatusOK, returnChirps)
		return
	}

	chirps, err := cfg.dbQueries.GetChirps(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	tmpChirps := make([]chirp, len(chirps))
	for i, v := range chirps {
		tmpChirps[i] = chirp{
			ID:        v.ID,
			CreatedAt: v.CreatedAt.String(),
			UpdatedAt: v.UpdatedAt.String(),
			Body:      v.Body,
			UserID:    v.UserID}
	}

	sortParam := r.URL.Query().Get("sort")
	returnChirps := tmpChirps
	sortParam = strings.ToLower(sortParam)
	if sortParam != "" && sortParam != "asc" && sortParam != "desc" {
		respondWithError(w, http.StatusBadRequest, "Invalid sort parameter")
		return
	}
	switch sortParam {
	case "asc":
		sort.Slice(tmpChirps, func(i, j int) bool {
			return tmpChirps[i].CreatedAt < tmpChirps[j].CreatedAt
		})
		break
	case "desc":
		sort.Slice(tmpChirps, func(i, j int) bool {
			return tmpChirps[i].CreatedAt > tmpChirps[j].CreatedAt
		})
		break
	}

	respondWithJSON(w, http.StatusOK, returnChirps)
}
func (cfg *apiConfig) handleChirps(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	w.Header().Set("Content-Type", "application/json")
	if r.Body == nil {
		respondWithError(w, http.StatusBadRequest, "No body")
		return
	}
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	userUuid, err := auth.ValidateJWT(bearerToken, cfg.svrToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}
	// fmt.Println(params)
	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	removeProfanity(&params.Body)
	createChirp, err := cfg.dbQueries.CreateChirp(r.Context(),
		database.CreateChirpParams{Body: params.Body, UserID: userUuid})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}
	respondWithJSON(w, http.StatusCreated,
		chirp{
			ID:        createChirp.ID,
			CreatedAt: createChirp.CreatedAt.String(),
			UpdatedAt: createChirp.UpdatedAt.String(),
			Body:      createChirp.Body,
			UserID:    createChirp.UserID})

}

func removeProfanity(chirp *string) {
	profane := []string{"kerfuffle", "sharbert", "fornax"}
	newChirp := *chirp

	c := strings.Split(newChirp, " ")
	for i, v := range c {
		for _, word := range profane {
			if strings.ToLower(v) == word {
				c[i] = strings.Repeat("*", 4)
			}
		}
	}
	newChirp = strings.Join(c, " ")

	*chirp = newChirp
}

func (cfg *apiConfig) handleGetChirpByID(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")

	w.Header().Set("Content-Type", "application/json")
	if r.Body == nil {
		respondWithError(w, http.StatusBadRequest, "No body")
		return
	}

	parsedChirpID, err := uuid.Parse(chirpID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	dbChirp, err := cfg.dbQueries.GetChirpById(r.Context(), parsedChirpID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "Chirp not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	returnChirp := chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt.String(),
		UpdatedAt: dbChirp.UpdatedAt.String(),
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}
	respondWithJSON(w, http.StatusOK, returnChirp)
}

func (cfg *apiConfig) handleChirpDelete(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")

	parsedChirpID, err := uuid.Parse(chirpID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	foundChirp, err := cfg.dbQueries.GetChirpById(r.Context(), parsedChirpID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	userUuid, err := auth.ValidateJWT(bearerToken, cfg.svrToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if foundChirp.UserID != userUuid {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	deleteChirpParams := database.DeleteChirpParams{ID: parsedChirpID, UserID: userUuid}
	err = cfg.dbQueries.DeleteChirp(r.Context(), deleteChirpParams)
	if err != nil {
		respondWithError(w, http.StatusForbidden, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	respondWithJSON(w, http.StatusNoContent, "")

}

func (cfg *apiConfig) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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

	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	userUuid, err := auth.ValidateJWT(bearerToken, cfg.svrToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if params.Email == "" {
		respondWithError(w, http.StatusUnauthorized, "Email is required")
		return
	}
	if params.Password == "" {
		respondWithError(w, http.StatusUnauthorized, "Password is required")
		return
	}

	_, err = cfg.dbQueries.GetUserById(r.Context(), userUuid)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
	}
	userParams := database.UpdateUserByIdParams{ID: userUuid, Email: params.Email, HashedPassword: hashedPassword}
	updatedUser, err := cfg.dbQueries.UpdateUserById(r.Context(), userParams)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	userResponse := userResponse{
		ID:           updatedUser.ID,
		CreatedAt:    updatedUser.CreatedAt.String(),
		UpdatedAt:    updatedUser.UpdatedAt.String(),
		Email:        updatedUser.Email,
		Token:        "",
		RefreshToken: "",
		IsChirpyRed:  updatedUser.IsChirpyRed,
	}
	respondWithJSON(w, http.StatusOK, userResponse)
}

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Chirpy/internal/auth"
	"github.com/Chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type user struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
	Email       string    `json:"email"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

type chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

type userResponse struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	platform := os.Getenv("PLATFORM")
	svrToken := os.Getenv("SVR_SECRET")
	polkaKey := os.Getenv("POLKA_KEY")

	ServeMux := http.NewServeMux()
	Server := http.Server{
		Addr:    ":8080",
		Handler: ServeMux,
	}
	fs := http.FileServer(http.Dir("."))
	cfg := &apiConfig{
		dbQueries: database.New(db),
		platform:  platform,
		svrToken:  svrToken,
		apiToken:  polkaKey}
	ServeMux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", fs)))
	ServeMux.HandleFunc("GET /admin/metrics", cfg.handlerMetrics)
	ServeMux.HandleFunc("GET /api/healthz", handleHealthz)
	ServeMux.HandleFunc("POST /admin/reset", cfg.handlerReset)
	ServeMux.HandleFunc("POST /api/users", cfg.handleUsers)
	ServeMux.HandleFunc("PUT /api/users", cfg.handleUserUpdate)
	ServeMux.HandleFunc("POST /api/chirps", cfg.handleChirps)
	ServeMux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handleGetChirpByID)
	ServeMux.HandleFunc("DELETE /api/chirps/{chirpID}", cfg.handleChirpDelete)
	ServeMux.HandleFunc("GET /api/chirps", cfg.handleGetChirps)
	ServeMux.HandleFunc("POST /api/login", cfg.handleLogin)
	ServeMux.HandleFunc("POST /api/refresh", cfg.handleRefresh)
	ServeMux.HandleFunc("POST /api/revoke", cfg.handleRevoke)
	ServeMux.HandleFunc("POST /api/polka/webhooks", cfg.handlePolka)
	err = Server.ListenAndServe()
	if err != nil {
		return
	}
}

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

func (cfg *apiConfig) handleGetChirps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	chirps, err := cfg.dbQueries.GetChirps(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	returnChirps := make([]chirp, len(chirps))
	for i, v := range chirps {
		returnChirps[i] = chirp{
			ID:        v.ID,
			CreatedAt: v.CreatedAt.String(),
			UpdatedAt: v.UpdatedAt.String(),
			Body:      v.Body,
			UserID:    v.UserID}
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

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.WriteHeader(code)
	response, err := json.Marshal(payload)
	// fmt.Println(string(dat))
	if err != nil {
		return
	}
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, responseString string) {
	type errorResp struct {
		Error string `json:"error"`
	}
	w.WriteHeader(code)
	errorResponse := errorResp{Error: responseString}
	dat, err := json.Marshal(errorResponse)
	if err != nil {
		return
	}
	_, err = w.Write(dat)
	if err != nil {
		return
	}
	return
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"status":"OK"}`))
	if err != nil {
		return
	}
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(fmt.Sprintf("<html>\n  <body>\n    <h1>Welcome, Chirpy Admin</h1>\n    <p>Chirpy has been visited %d times!</p>\n  </body>\n</html>", cfg.fileserverHits.Load())))
	if err != nil {
		return
	}
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

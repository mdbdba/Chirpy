package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/Chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type user struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
	Email     string    `json:"email"`
}

type chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	platform := os.Getenv("PLATFORM")

	ServeMux := http.NewServeMux()
	Server := http.Server{
		Addr:    ":8080",
		Handler: ServeMux,
	}
	fs := http.FileServer(http.Dir("."))
	cfg := &apiConfig{dbQueries: database.New(db), platform: platform}
	ServeMux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", fs)))
	ServeMux.HandleFunc("GET /admin/metrics", cfg.handlerMetrics)
	ServeMux.HandleFunc("GET /api/healthz", handleHealthz)
	ServeMux.HandleFunc("POST /admin/reset", cfg.handlerReset)
	ServeMux.HandleFunc("POST /api/users", cfg.handleUsers)
	ServeMux.HandleFunc("POST /api/chirps", cfg.handleChirps)
	err = Server.ListenAndServe()
	if err != nil {
		return
	}
}

func (cfg *apiConfig) handleChirps(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string `json:"body"`
		UserId string `json:"user_id"`
	}

	type cleaned struct {
		CleanedBody string `json:"cleaned_body"`
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
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}
	// fmt.Println(params)
	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	removeProfanity(&params.Body)
	parsedUUID, err := uuid.Parse(params.UserId)
	if err != nil {
		return
	}

	createChirp, err := cfg.dbQueries.CreateChirp(r.Context(),
		database.CreateChirpParams{Body: params.Body, UserID: parsedUUID})
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
		Email string `json:"email"`
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
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
	}

	createUser, err := cfg.dbQueries.CreateUser(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	respondWithJSON(w, http.StatusCreated,
		user{ID: createUser.ID, CreatedAt: createUser.CreatedAt.String(),
			UpdatedAt: createUser.UpdatedAt.String(),
			Email:     createUser.Email})
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	err := cfg.dbQueries.DeleteUsers(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	respondWithJSON(w, http.StatusOK, "Reset")
}

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			cfg.fileserverHits.Add(1)
			next.ServeHTTP(w, r)
		},
	)
}

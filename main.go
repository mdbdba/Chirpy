package main

import (
	"database/sql"
	"net/http"
	"os"

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

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"status":"OK"}`))
	if err != nil {
		return
	}
}

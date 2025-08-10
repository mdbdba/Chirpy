package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
)

func main() {

	ServeMux := http.NewServeMux()
	Server := http.Server{
		Addr:    ":8080",
		Handler: ServeMux,
	}
	fs := http.FileServer(http.Dir("."))
	cfg := &apiConfig{}
	ServeMux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", fs)))
	ServeMux.HandleFunc("GET /admin/metrics", cfg.handlerMetrics)
	ServeMux.HandleFunc("GET /api/healthz", handleHealthz)
	ServeMux.HandleFunc("POST /admin/reset", cfg.handlerReset)
	ServeMux.HandleFunc("POST /api/validate_chirp", handleValidate)
	err := Server.ListenAndServe()
	if err != nil {
		return
	}
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	type valid struct {
		Valid bool `json:"valid"`
	}

	w.Header().Set("Content-Type", "application/json")
	if r.Body == nil {
		buildErrorResponse(w, "No body")
		return
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		buildErrorResponse(w, "Something went wrong")
		return
	}
	// fmt.Println(params)
	if len(params.Body) > 140 {
		buildErrorResponse(w, "Chirp is too long")
		return
	}
	w.WriteHeader(http.StatusOK)
	validResponse := valid{Valid: true}
	dat, err := json.Marshal(validResponse)
	// fmt.Println(string(dat))
	if err != nil {
		return
	}
	w.Write(dat)
}

func buildErrorResponse(w http.ResponseWriter, responseString string) {
	type errorResp struct {
		Error string `json:"error"`
	}

	w.WriteHeader(http.StatusBadRequest)
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

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			cfg.fileserverHits.Add(1)
			next.ServeHTTP(w, r)
		},
	)
}

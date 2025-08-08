package main

import (
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
	ServeMux.HandleFunc("/metrics", cfg.handlerMetrics)
	ServeMux.HandleFunc("/healthz", handleHealthz)
	ServeMux.HandleFunc("/reset", cfg.handlerReset)
	err := Server.ListenAndServe()
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

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
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

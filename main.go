package main

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	apiCfg := &apiConfig{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", appHandler)

	fileServer := http.StripPrefix(
		"/app",
		http.FileServer(http.Dir(".")),
	)
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(fileServer))
	mux.HandleFunc("GET /metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /reset", apiCfg.resetHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	server.ListenAndServe()
}

func appHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hits: %d", cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

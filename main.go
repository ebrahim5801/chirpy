package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

type validate struct {
	Body string `json:"body"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type validResponse struct {
	CleanedBody string `json:"cleaned_body"`
}

func main() {
	apiCfg := &apiConfig{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", appHandler)

	fileServer := http.StripPrefix(
		"/app",
		http.FileServer(http.Dir(".")),
	)

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(fileServer))
	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	mux.HandleFunc("POST /api/validate_chirp", apiCfg.validateHandler)

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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
		</html>`, cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) validateHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	decoder := json.NewDecoder(req.Body)

	data := &validate{}

	err := decoder.Decode(data)
	if err != nil {
		errData := &errorResponse{Error: "Something went wrong"}
		dat, err := json.Marshal(errData)
		if err != nil {
			log.Printf("Error marshalling data: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(dat)
		return

	}

	if len(data.Body) > 140 {
		errData := &errorResponse{Error: "Chirp is too long"}
		dat, err := json.Marshal(errData)
		if err != nil {
			log.Printf("Error marshalling data: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(dat)
		return

	}

	cleanedBody := replaceProfaneWords(data.Body)

	validData := &validResponse{CleanedBody: cleanedBody}
	response, err := json.Marshal(validData)
	if err != nil {
		errData := &errorResponse{Error: "Something went wrong"}
		response, err := json.Marshal(errData)
		if err != nil {
			log.Printf("Error marshalling data: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(response)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func replaceProfaneWords(text string) string {
	profaneWords := map[string]bool{
		"kerfuffle": true,
		"sharbert":  true,
		"fornax":    true,
	}

	words := strings.Split(text, " ")

	for i, word := range words {
		lowerWord := strings.ToLower(word)

		if profaneWords[lowerWord] {
			words[i] = "****"
		}
	}

	return strings.Join(words, " ")
}

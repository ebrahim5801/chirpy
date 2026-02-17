package main

import (
	"io"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", appHandler)

	fileServer := http.StripPrefix(
		"/app",
		http.FileServer(http.Dir(".")),
	)
	mux.Handle("/app/", fileServer)

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

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/natet/honeygen/backend/internal/config"
)

func main() {
	addr := config.EnvOrDefault("HTTP_ADDR", ":8080")
	sqlitePath := config.EnvOrDefault("SQLITE_PATH", "/app/storage/sqlite/honeygen.db")
	generatedAssetsDir := config.EnvOrDefault("GENERATED_ASSETS_DIR", "/app/storage/generated")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "honeygen api skeleton\nsqlite=%s\ngenerated-assets=%s\n", sqlitePath, generatedAssetsDir)
	})

	log.Printf("api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

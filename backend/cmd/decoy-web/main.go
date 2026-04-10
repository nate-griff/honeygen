package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := envOrDefault("HTTP_ADDR", ":8081")
	generatedAssetsDir := envOrDefault("GENERATED_ASSETS_DIR", "/app/storage/generated")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, "<!doctype html><html><head><title>Decoy Web</title></head><body><h1>Decoy web skeleton</h1><p>Generated assets directory: <code>%s</code></p></body></html>", generatedAssetsDir)
	})
	mux.Handle("/generated/", http.StripPrefix("/generated/", http.FileServer(http.Dir(generatedAssetsDir))))

	log.Printf("decoy-web listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

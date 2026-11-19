// Command backend starts the HTTP server that serves the web panel and a few
// small JSON endpoints reserved for future plugin support.
package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/idealisan/home-lab-core/src"
)

func main() {
	addr := envOr("HOME_LAB_ADDR", ":8080")

	frontend, err := fs.Sub(src.FrontendFS, "frontend")
	if err != nil {
		log.Fatalf("frontend assets: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/plugins", pluginsHandler)
	mux.Handle("/", http.FileServer(http.FS(frontend)))

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Home Lab Core listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"name":   "home-lab-core",
	})
}

// pluginsHandler is a placeholder for the Lua plugin registry. The menu bar is
// designed to render plugin entries from this endpoint once plugins exist.
func pluginsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]string{})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

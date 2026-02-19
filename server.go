package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

//go:embed templates/index.html
var templateFS embed.FS

// ServeReport starts a local HTTP server on the given port.
// It re-reads and re-aggregates the data on every /api/report request so
// the dashboard stays live as new Claude Code sessions are written.
func ServeReport(claudeDir string, opts AggregateOptions, port int) error {
	mux := http.NewServeMux()

	// Serve the web UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := templateFS.ReadFile("templates/index.html")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// Re-compute the report on every request so new sessions are picked up.
	mux.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		files, err := DiscoverFiles(claudeDir)
		if err != nil {
			http.Error(w, "failed to discover files: "+err.Error(), 500)
			return
		}
		opts.StatsCache = ParseStatsCache(claudeDir)
		report := Aggregate(files, opts)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(report)
	})

	addr := fmt.Sprintf(":%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)

	fmt.Printf("Starting web UI at %s\n", url)
	fmt.Println("Press Ctrl+C to stop.")

	// Open browser after a short delay (let the server start first)
	go func() {
		time.Sleep(300 * time.Millisecond)
		openBrowser(url)
	}()

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return server.ListenAndServe()
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}

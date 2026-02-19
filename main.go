package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	days := flag.Int("days", 0, "Limit analysis to last N days (0 = all time)")
	project := flag.String("project", "", "Filter by project name substring")
	jsonOut := flag.Bool("json", false, "Output machine-readable JSON to stdout")
	serve := flag.Bool("serve", false, "Start local web UI server")
	port := flag.Int("port", 8080, "Port for web UI server (used with --serve)")
	claudeDir := flag.String("claude-dir", "", "Path to Claude data directory (default: ~/.claude)")
	flag.Parse()

	// Resolve Claude directory
	dir := *claudeDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot find home directory: %v\n", err)
			os.Exit(1)
		}
		dir = filepath.Join(home, ".claude")
	}

	if _, err := os.Stat(dir); err != nil {
		fmt.Fprintf(os.Stderr, "error: Claude data directory not found at %s\n", dir)
		fmt.Fprintf(os.Stderr, "Use --claude-dir to specify an alternate path.\n")
		os.Exit(1)
	}

	opts := AggregateOptions{
		Days:    *days,
		Project: *project,
	}

	// --serve: hand off to the HTTP server, which re-aggregates on each request.
	if *serve {
		if err := ServeReport(dir, opts, *port); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Terminal / JSON modes: aggregate once.
	files, err := DiscoverFiles(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error discovering files: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No JSONL session files found. Have you used Claude Code yet?")
		os.Exit(0)
	}

	opts.StatsCache = ParseStatsCache(dir)
	report := Aggregate(files, opts)

	if report.Grand.TotalTokens() == 0 {
		if *days > 0 {
			fmt.Fprintf(os.Stderr, "No token data found in the last %d days.\n", *days)
		} else {
			fmt.Fprintln(os.Stderr, "No token data found.")
		}
		os.Exit(0)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
			os.Exit(1)
		}
	} else {
		PrintReport(os.Stdout, report, isTerminal())
	}
}

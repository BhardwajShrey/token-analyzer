package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	uuidRegex    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	agentIDRegex = regexp.MustCompile(`^agent-[0-9a-f]+\.jsonl$`)
)

// DiscoverFiles walks the ~/.claude/projects/ directory and returns
// all classified JSONL session and subagent files.
func DiscoverFiles(claudeDir string) ([]FileInfo, error) {
	projectsDir := filepath.Join(claudeDir, "projects")

	var files []FileInfo

	err := filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}

		rel, err := filepath.Rel(projectsDir, path)
		if err != nil {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))

		switch {
		case len(parts) == 2:
			// <slug>/<uuid>.jsonl
			base := parts[1]
			uuidStr := strings.TrimSuffix(base, ".jsonl")
			if uuidRegex.MatchString(uuidStr) {
				files = append(files, FileInfo{
					Path:        path,
					Kind:        KindSession,
					ProjectSlug: parts[0],
					SessionID:   uuidStr,
				})
			}

		case len(parts) == 4 && parts[2] == "subagents" && agentIDRegex.MatchString(parts[3]):
			// <slug>/<uuid>/subagents/agent-<id>.jsonl
			agentID := strings.TrimSuffix(parts[3], ".jsonl")
			files = append(files, FileInfo{
				Path:        path,
				Kind:        KindSubagent,
				ProjectSlug: parts[0],
				SessionID:   parts[1],
				AgentID:     agentID,
			})
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return files, nil
}

// ParseStatsCache reads ~/.claude/stats-cache.json.
// Returns nil if the file is missing or malformed.
func ParseStatsCache(claudeDir string) *StatsCache {
	path := filepath.Join(claudeDir, "stats-cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var sc StatsCache
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil
	}
	return &sc
}

// slugToPath converts a project slug like "-Users-foo-bar" to "/Users/foo/bar".
// This is a best-effort fallback — use cwd from parsed records when available.
func slugToPath(slug string) string {
	if slug == "" {
		return ""
	}
	return "/" + strings.TrimPrefix(slug, "-")
}

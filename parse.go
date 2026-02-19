package main

import (
	"bufio"
	"encoding/json"
	"os"
)

// ParseFile reads a JSONL file and returns all assistant-type records
// that contain non-zero token usage. Malformed lines are silently skipped
// and counted in the returned parseErrors count.
// Records are deduplicated by UUID.
func ParseFile(path string) (records []MessageRecord, parseErrors int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 1
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// 10 MB buffer — session files can contain large inline content
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	seen := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec MessageRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			parseErrors++
			continue
		}

		// Only assistant records carry token usage
		if rec.Type != "assistant" {
			continue
		}

		// Skip zero-usage records (streaming prefix acknowledgments)
		if rec.Message.Usage.IsZero() {
			continue
		}

		// Deduplicate by UUID
		if rec.UUID != "" {
			if seen[rec.UUID] {
				continue
			}
			seen[rec.UUID] = true
		}

		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		parseErrors++
	}

	return records, parseErrors
}

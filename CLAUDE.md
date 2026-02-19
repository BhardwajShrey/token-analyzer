# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o token-analyzer .

# Run terminal report (all time)
./token-analyzer

# Run with filters
./token-analyzer --days 7
./token-analyzer --project <name-substring>

# JSON output
./token-analyzer --json | jq '.Grand.CostUSD'

# Web UI (opens browser at http://localhost:8080)
./token-analyzer --serve
./token-analyzer --serve --port 9000

# Custom Claude data directory
./token-analyzer --claude-dir /path/to/.claude
```

## Architecture

Pure stdlib Go CLI. No external Go dependencies. The web UI uses Chart.js via CDN.

**Data flow:** `discover.go` walks `~/.claude/projects/` → `parse.go` reads each JSONL file → `aggregate.go` builds multi-axis summaries → `report.go` (terminal) or `server.go` (web) renders output.

**File roles:**
- `models.go` — All data types. `UsageTotals` is the core accumulator used everywhere.
- `pricing.go` — Model family pricing table. Uses longest-prefix matching on model IDs (e.g., `claude-sonnet-4-5-20250929` matches family prefix `claude-sonnet-4`).
- `discover.go` — File classification: session files at `<slug>/<uuid>.jsonl`, subagent files at `<slug>/<uuid>/subagents/agent-<id>.jsonl`. Also reads `stats-cache.json` for the peak-hour insight.
- `parse.go` — Reads JSONL with a 10 MB scanner buffer; keeps only `type == "assistant"` records with non-zero usage; deduplicates by `uuid`.
- `aggregate.go` — Accumulates into `projectMap`, `sessionMap`, `dailyMap`, `modelMap`; generates `[]Insight` after aggregation.
- `server.go` — `net/http` server with `go:embed` for the HTML template; `/api/report` serves the `AggregatedReport` as JSON.
- `templates/index.html` — Single-page app; fetches `/api/report` on load; uses Chart.js for the stacked bar daily trend chart.

**Critical parsing detail:** Token counts live at `record.Message.Usage` (the nested `message` object), NOT at a top-level `usage` field (which is always null in the JSONL files).

**Session vs subagent tokens:** Subagent records accumulate into `SessionSummary.SubagentTotals` separately so overhead can be reported distinctly from the main conversation.

# token-analyzer

A CLI tool that parses your local Claude Code session logs and gives you a breakdown of token usage, costs, and cache efficiency — with both a terminal report and a live web dashboard.

## Usage

```bash
go build -o token-analyzer .

# Terminal report (all time)
./token-analyzer

# Last 7 days only
./token-analyzer --days 7

# Filter to a specific project
./token-analyzer --project my-app

# Machine-readable JSON
./token-analyzer --json | jq '.Grand.CostUSD'

# Live web dashboard (opens browser at http://localhost:8080)
./token-analyzer --serve

# Custom port
./token-analyzer --serve --port 9000

# Custom Claude data directory (default: ~/.claude)
./token-analyzer --claude-dir /path/to/.claude
```

## What it shows

**Terminal report:**
- Token breakdown: input, output, cache writes, cache reads — with percentages
- Cache efficiency score and color-coded bar
- Estimated cost per model
- Projects ranked by token consumption
- Top sessions with subagent overhead separated out
- Daily trend sparkline (last 30 days)
- Actionable insights (cache efficiency, verbose responses, subagent overhead, peak hour)

**Web dashboard (`--serve`):**
- Summary cards for total tokens, cache efficiency, cost, session count
- Interactive stacked bar chart of daily token usage (input / output / cache write / cache read)
- Model, project, and session tables
- Color-coded insight cards
- Auto-refreshes every 30 seconds to reflect new sessions as you work

## How it works

Claude Code writes a JSONL file for every session under `~/.claude/projects/<project-slug>/`. Each line is a message record; assistant messages include token usage counts. This tool discovers all those files across every project on your machine, parses them, and aggregates by project, session, model, and day.

Token counts come from `record.message.usage` in the JSONL files. The `stats-cache.json` is used only for the peak-hour insight.

## Notes

- **Coverage**: only sessions whose JSONL files still exist under `~/.claude/projects/` are counted. The `stats-cache.json` may show higher historical totals for sessions that have since been removed.
- **Costs**: estimated using Anthropic's published per-model pricing. Unknown model IDs are flagged in insights and counted as $0.
- **No writes**: the tool is read-only and never modifies your Claude data directory.

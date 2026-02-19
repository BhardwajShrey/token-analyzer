# token-analyzer

A CLI tool that parses your local Claude Code session logs and gives you a breakdown of token usage, costs, cache efficiency, and **prompt clarity** — with both a terminal report and a live web dashboard.

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
- **Prompt Clarity section** with score, weekly trend, and per-metric good/ok/warn labels
- **Coaching Tip section** with a targeted technique and before/after prompt example

**Web dashboard (`--serve`):**
- Summary cards for total tokens, cache efficiency, cost, session count
- Interactive stacked bar chart of daily token usage (input / output / cache write / cache read)
- Model, project, and session tables
- Color-coded insight cards
- Auto-refreshes every 30 seconds to reflect new sessions as you work
- **Hover tooltips** on every metric label and table header explaining what each number means
- **Good/ok/warn indicators** with one-liner explanations on summary cards
- **Prompt Clarity section** with composite score, weekly line chart, and per-metric breakdown
- **Coaching Tip card** with side-by-side weak/strong prompt examples

## Prompt Clarity

The tool heuristically scores how well-specified your prompts are, across three signals:

| Metric | What it measures | Good direction |
|---|---|---|
| **Correction Rate** | % of messages that walk back a prior request | ↓ lower |
| **Clarification Rate** | % of sessions where the model asked a clarifying question first | ↓ lower |
| **Front-load Ratio** | % of your prompt text sent in the first message | ↑ higher |
| **Clarity Score** | Composite 0–100 weighted across the three signals | ↑ higher |

Score formula: `100 × (0.40 × front_load + 0.35 × (1 − correction_rate) + 0.25 × (1 − clarification_rate))`

Weekly trends are tracked so you can see whether your prompting discipline is improving over time.

## Coaching Tips

After measuring your clarity signals, the tool identifies your single weakest metric and surfaces a concrete, actionable tip — including a technique explanation and a realistic before/after prompt example. Tips rotate weekly (by ISO week number) so you always have something new to try.

The weakest metric is chosen by normalized gap-to-good: whichever signal is furthest from its "good" threshold drives the tip. The tip is omitted entirely when all three signals are green.

Example tip (Front-load Ratio at 51%):

> **Lead with the relevant code**
>
> If you are referencing code, paste it in the opening prompt rather than waiting for the model to ask.
>
> ✗ `"Can you improve the performance here?"` → `[next turn] "Here's the hot path: ..."`
>
> ✓ `"Optimize this hot path for latency. [paste function] Reduce allocations. Keep the same external interface."`

## How it works

Claude Code writes a JSONL file for every session under `~/.claude/projects/<project-slug>/`. Each line is a message record; assistant messages include token usage counts. This tool discovers all those files across every project on your machine, parses them, and aggregates by project, session, model, and day.

Token counts come from `record.message.usage` in the JSONL files. The `stats-cache.json` is used only for the peak-hour insight.

## Notes

- **Coverage**: only sessions whose JSONL files still exist under `~/.claude/projects/` are counted. The `stats-cache.json` may show higher historical totals for sessions that have since been removed.
- **Costs**: estimated using Anthropic's published per-model pricing. Unknown model IDs are flagged in insights and counted as $0.
- **No writes**: the tool is read-only and never modifies your Claude data directory.

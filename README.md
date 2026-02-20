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

## Correction Taxonomy

Walk-backs are classified into three types so the coaching tip targets the specific habit driving your correction rate:

| Type | What it means | Example signal |
|---|---|---|
| **Scope** | You changed more than you meant to | "don't touch", "only that file", "leave the interface" |
| **Format** | The output medium or length was wrong | "just the code", "as JSON", "no explanation" |
| **Intent** | The model misunderstood what was asked | "that's not what I meant", "you misunderstood", "let me rephrase" |

Detection looks at the first 200 characters of each follow-up message. Scope and Format corrections require a walk-back signal (e.g. "actually", "scratch that") plus a type phrase in the same message. Intent signals fire standalone — they are inherently retroactive.

When corrections are detected, the terminal report shows a type tree under Correction Rate:

```
  Correction Rate       28.0%  ↓ lower is better  [warn]
    ├─ Scope    16.0%
    ├─ Format   10.0%
    └─ Intent    2.0%
```

## Coaching Tips

After measuring your clarity signals, the tool identifies your weakest metric and surfaces actionable tips — each with a technique explanation and a realistic before/after prompt example. Tips are randomised on every run so you see something different each time you open the dashboard.

When correction rate is your weakest metric, you get **one tip per detected correction type** (scope, format, intent) rather than a single generic tip. Each tip is drawn from a type-specific bank targeting the exact habit causing that class of walk-back.

The weakest metric is chosen by normalized gap-to-good: whichever signal is furthest from its "good" threshold drives the tip. Tips are omitted entirely when all three signals are green.

Example tip (Scope corrections at 16%):

> **Write a constraints block**
>
> Add a dedicated constraints block to every prompt: list files that are off-limits, interfaces that must stay intact, and folders that are read-only.
>
> ✗ `"Refactor the parser"`
>
> ✓ `"Refactor parseConfig in config/parser.go to reduce nesting. Constraints: only modify config/parser.go. Do not touch config/types.go, any test files, or the public ParseConfig signature."`

## How it works

Claude Code writes a JSONL file for every session under `~/.claude/projects/<project-slug>/`. Each line is a message record; assistant messages include token usage counts. This tool discovers all those files across every project on your machine, parses them, and aggregates by project, session, model, and day.

Token counts come from `record.message.usage` in the JSONL files. The `stats-cache.json` is used only for the peak-hour insight.

## Notes

- **Coverage**: only sessions whose JSONL files still exist under `~/.claude/projects/` are counted. The `stats-cache.json` may show higher historical totals for sessions that have since been removed.
- **Costs**: estimated using Anthropic's published per-model pricing. Unknown model IDs are flagged in insights and counted as $0.
- **No writes**: the tool is read-only and never modifies your Claude data directory.

package main

import "time"

// CoachingTip is a single actionable nudge tied to the user's weakest clarity metric.
type CoachingTip struct {
	Metric    string // "correction_rate" | "clarification_rate" | "front_load_ratio"
	Level     string // "ok" | "warn"
	Headline  string // short imperative phrase
	Technique string // 2–3 sentence explanation
	WeakEx    string // example of a weak prompt (newlines separate turns)
	StrongEx  string // example of a strong prompt
}

// tipBank maps "<metric>_<level>" to a slice of 2 tips that rotate weekly.
var tipBank = map[string][]CoachingTip{
	"correction_rate_warn": {
		{
			Metric:    "correction_rate",
			Level:     "warn",
			Headline:  "Write a spec comment first",
			Technique: "Before typing your request, jot down three things: what you want done, the format the output should take, and what must not change. Paste that as the opening of your prompt — it forces precision before you engage the model.",
			WeakEx:    "Clean up this function",
			StrongEx:  "Refactor parseConfig to reduce nesting.\nMax 2 levels deep. Keep the function signature unchanged.\nDo not alter any test files.",
		},
		{
			Metric:    "correction_rate",
			Level:     "warn",
			Headline:  "State all constraints upfront",
			Technique: "Corrections usually happen because the model solved the right problem with the wrong constraints. List every hard constraint in your first message: language version, existing interfaces to preserve, performance requirements, things to avoid.",
			WeakEx:    "Add authentication to the API",
			StrongEx:  "Add JWT auth to the /api/ routes using middleware/auth.go.\nGo 1.21, no new dependencies. Do not modify the DB schema.\nPreserve all existing handler signatures.",
		},
	},
	"correction_rate_ok": {
		{
			Metric:    "correction_rate",
			Level:     "ok",
			Headline:  "Add a 'do not' line",
			Technique: "One explicit 'Do not X' at the end of your prompt prevents the most common walk-back. Think about what the model typically does that you then have to correct, and ban it upfront.",
			WeakEx:    "Improve error handling in the parser",
			StrongEx:  "Improve error handling in ParseFile.\nWrap errors with fmt.Errorf for context.\nDo not change the function's return types.",
		},
		{
			Metric:    "correction_rate",
			Level:     "ok",
			Headline:  "End with the acceptance criterion",
			Technique: "Add one sentence describing the exact outcome that satisfies you: 'Done when X'. This gives the model a clear stopping condition and reduces the overshoot that leads to corrections.",
			WeakEx:    "Make the tests faster",
			StrongEx:  "Parallelize the test suite in parse_test.go using t.Parallel().\nDone when go test ./... completes in under 2 seconds.\nDo not change any test assertions.",
		},
	},
	"clarification_rate_warn": {
		{
			Metric:    "clarification_rate",
			Level:     "warn",
			Headline:  "Specify output format explicitly",
			Technique: "When the model asks a clarifying question it is usually about scope or format. Pre-empt this by ending your prompt with: 'Return X in format Y. Skip Z.' This removes the ambiguity before the model has to ask.",
			WeakEx:    "Explain how caching works",
			StrongEx:  "Explain prompt caching in 3 bullet points.\nCover: when cache hits occur and their cost impact.\nSkip API-level mechanics and pricing tables.",
		},
		{
			Metric:    "clarification_rate",
			Level:     "warn",
			Headline:  "Add a scope boundary",
			Technique: "Vague prompts invite clarifying questions. Add one sentence defining the boundary: what is in scope and what is out. 'Only modify X, leave Y unchanged' eliminates most ambiguity in one line.",
			WeakEx:    "Fix the bug in the parser",
			StrongEx:  "Fix the nil pointer dereference in ParseFile at line 43.\nModify only that function.\nDo not refactor surrounding code.",
		},
	},
	"clarification_rate_ok": {
		{
			Metric:    "clarification_rate",
			Level:     "ok",
			Headline:  "State the output medium",
			Technique: "Many clarifying questions are about format. State whether you want code, prose, a table, or bullet points. 'Show me as a code example' vs. 'explain in prose' eliminates a whole class of back-and-forth.",
			WeakEx:    "Show me how to handle errors in Go",
			StrongEx:  "Show Go error handling as a code snippet.\nCover: fmt.Errorf wrapping, sentinel errors, custom types.\nNo prose — code only.",
		},
		{
			Metric:    "clarification_rate",
			Level:     "ok",
			Headline:  "Give a length budget",
			Technique: "Ambiguity about depth causes clarifying questions. Telling the model how much you want — 'one paragraph', '5 bullet points', 'a single function' — gives it a concrete scope to work within.",
			WeakEx:    "Summarize what this code does",
			StrongEx:  "Summarize what main.go does in 3 bullet points.\nAudience: a new contributor.\nNo implementation details.",
		},
	},
	"front_load_ratio_warn": {
		{
			Metric:    "front_load_ratio",
			Level:     "warn",
			Headline:  "Paste everything in the first message",
			Technique: "Context trickled in across turns cannot be cached and forces the model to re-process state it has already seen. Assemble all relevant code, file paths, and constraints into one opening prompt — long first messages are better than short back-and-forth.",
			WeakEx:    "Update the parser\n[next turn] Here's the relevant code: ...\n[next turn] Oh and don't change the interface",
			StrongEx:  "Update ParseFile in parse.go to handle empty lines gracefully.\n[paste full function]\nDo not change the function signature.",
		},
		{
			Metric:    "front_load_ratio",
			Level:     "warn",
			Headline:  "Use a prompt template",
			Technique: "A four-part structure prevents context from leaking out gradually. Use: Task (one sentence) → Context (relevant code or files) → Constraints (what must not change) → Output (format you expect).",
			WeakEx:    "Make the tests pass",
			StrongEx:  "Task: fix 3 failing tests in parse_test.go.\nContext: [paste tests + ParseFile].\nConstraints: do not alter test assertions.\nOutput: only the corrected ParseFile function.",
		},
	},
	"front_load_ratio_ok": {
		{
			Metric:    "front_load_ratio",
			Level:     "ok",
			Headline:  "Lead with the relevant code",
			Technique: "If you are referencing code, paste it in the opening prompt rather than waiting for the model to ask. The model handles more context than you might expect, and a complete first message means better cache utilization on every follow-up.",
			WeakEx:    "Can you improve the performance here?\n[next turn] Here's the hot path: ...",
			StrongEx:  "Optimize this hot path for latency. [paste function]\nReduce allocations. Keep the same external interface.",
		},
		{
			Metric:    "front_load_ratio",
			Level:     "ok",
			Headline:  "Paste error output with the question",
			Technique: "When debugging, the error message, relevant stack trace, and surrounding code all belong in the first message — not discovered through follow-ups. The model solves it in one shot when the evidence is already there.",
			WeakEx:    "Why is my test failing?",
			StrongEx:  "Why is TestParseFile failing? Error:\n[paste error output]\nRelevant code:\n[paste function + test case]",
		},
	},
}

// SelectCoachingTip returns the most relevant tip for the user's weakest clarity
// metric, rotating weekly. Returns nil if all metrics are good or data is
// insufficient.
func SelectCoachingTip(r *ClarityReport) *CoachingTip {
	if r == nil || r.SessionCount < 2 {
		return nil
	}

	o := r.Overall

	// All three signals good → no coaching needed
	ci := CorrectionRateInsight(o.CorrectionRate)
	cli := ClarificationRateInsight(o.ClarificationRate)
	fi := FrontLoadRatioInsight(o.FrontLoadRatio)
	if ci.Level == "good" && cli.Level == "good" && fi.Level == "good" {
		return nil
	}

	// Normalized gap-to-good: how far each metric is from its "good" threshold
	corrGap := o.CorrectionRate - 0.10
	if corrGap < 0 {
		corrGap = 0
	}
	clarGap := o.ClarificationRate - 0.15
	if clarGap < 0 {
		clarGap = 0
	}
	frontGap := (0.60 - o.FrontLoadRatio) / 0.60
	if frontGap < 0 {
		frontGap = 0
	}

	// Pick the weakest metric
	var worstMetric, worstLevel string
	if corrGap >= clarGap && corrGap >= frontGap {
		worstMetric = "correction_rate"
		worstLevel = ci.Level
	} else if clarGap >= frontGap {
		worstMetric = "clarification_rate"
		worstLevel = cli.Level
	} else {
		worstMetric = "front_load_ratio"
		worstLevel = fi.Level
	}

	// "good" metrics don't get a coaching tip
	if worstLevel == "good" {
		return nil
	}

	key := worstMetric + "_" + worstLevel
	tips, ok := tipBank[key]
	if !ok || len(tips) == 0 {
		// fallback: try warn tier
		key = worstMetric + "_warn"
		tips = tipBank[key]
		if len(tips) == 0 {
			return nil
		}
	}

	// Rotate by ISO week so the tip changes each Monday
	_, week := time.Now().ISOWeek()
	tip := tips[week%len(tips)]
	return &tip
}

// computeWeekDelta returns the score change between the two most recent weeks.
// Returns nil if fewer than 2 weekly entries exist.
func computeWeekDelta(weekly []WeeklyClarity) *float64 {
	if len(weekly) < 2 {
		return nil
	}
	d := weekly[len(weekly)-1].Score - weekly[len(weekly)-2].Score
	return &d
}

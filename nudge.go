package main

import "math/rand"

// CoachingTip is a single actionable nudge tied to the user's weakest clarity metric.
type CoachingTip struct {
	Metric    string // "correction_rate" | "clarification_rate" | "front_load_ratio"
	SubMetric string // "scope" | "format" | "intent" — empty for non-correction tips
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
	"correction_scope_warn": {
		{
			Metric:    "correction_rate",
			SubMetric: "scope",
			Level:     "warn",
			Headline:  "Write a constraints block",
			Technique: "Scope corrections happen when the model touches files or interfaces you didn't want changed. Add a dedicated constraints block to every prompt: list files that are off-limits, interfaces that must stay intact, and folders that are read-only. Put it right after your task description so it's impossible to miss.",
			WeakEx:    "Refactor the parser",
			StrongEx:  "Refactor parseConfig in config/parser.go to reduce nesting.\nConstraints: only modify config/parser.go. Do not touch config/types.go, any test files, or the public ParseConfig signature.",
		},
		{
			Metric:    "correction_rate",
			SubMetric: "scope",
			Level:     "warn",
			Headline:  "Scope the task by file, not topic",
			Technique: "Describing a task by topic ('fix error handling') leaves scope ambiguous — the model may touch every file related to that topic. Instead, name the exact file and function. 'Fix error handling in ParseFile in parse.go' makes the boundary unambiguous without any extra constraints line.",
			WeakEx:    "Fix the error handling across the codebase",
			StrongEx:  "Fix error handling in ParseFile in parse.go only.\nWrap errors with fmt.Errorf. Do not modify any other files.",
		},
	},
	"correction_scope_ok": {
		{
			Metric:    "correction_rate",
			SubMetric: "scope",
			Level:     "ok",
			Headline:  "List what must stay unchanged",
			Technique: "Even when corrections are infrequent, listing the untouchable parts up front avoids the walk-back entirely. One line — 'Do not modify X, Y, or Z' — gives the model a clear boundary and lets it focus on what you actually want changed.",
			WeakEx:    "Add a cache to the session loader",
			StrongEx:  "Add an in-memory cache to loadSession in session.go.\nDo not modify the Session struct, any callers, or test files.",
		},
		{
			Metric:    "correction_rate",
			SubMetric: "scope",
			Level:     "ok",
			Headline:  "Use 'Only X, nothing else'",
			Technique: "The phrase 'only X, nothing else' is the most compact scope constraint. It fits in any prompt without adding bulk and leaves zero ambiguity about how far the change should spread.",
			WeakEx:    "Update the README",
			StrongEx:  "Update the Installation section in README.md only, nothing else.\nAdd a note about the --serve flag.",
		},
	},
	"correction_format_warn": {
		{
			Metric:    "correction_rate",
			SubMetric: "format",
			Level:     "warn",
			Headline:  "Specify output medium first",
			Technique: "Format corrections mean the model returned prose when you wanted code, or a block when you wanted inline. Put the output medium in the very first sentence — before the task description. 'Show me as a code snippet: ...' is harder to misread than burying the format requirement at the end.",
			WeakEx:    "How do I implement a retry loop in Go?",
			StrongEx:  "Show me as a Go code snippet: a retry loop with exponential backoff.\nNo prose — code only. Use stdlib only.",
		},
		{
			Metric:    "correction_rate",
			SubMetric: "format",
			Level:     "warn",
			Headline:  "End with a format line",
			Technique: "If you can't lead with the format, always close with one explicit format line. 'Return only the modified function, no explanation' at the end of any prompt prevents the most common format walk-back: the model returning a full file or a wall of prose around a small change.",
			WeakEx:    "Update the error message in validateInput",
			StrongEx:  "Update the error message in validateInput to include the field name.\nReturn only the modified function, no explanation.",
		},
	},
	"correction_format_ok": {
		{
			Metric:    "correction_rate",
			SubMetric: "format",
			Level:     "ok",
			Headline:  "Name the anti-format explicitly",
			Technique: "Saying what you don't want is as effective as saying what you do. 'No prose', 'no comments', 'no markdown' each eliminate a whole class of unwanted output in one word. Add one anti-format line to any prompt where you've historically walked back the format.",
			WeakEx:    "Summarize how the cache works",
			StrongEx:  "Summarize how the cache works in 3 bullet points. No prose, no headers.",
		},
		{
			Metric:    "correction_rate",
			SubMetric: "format",
			Level:     "ok",
			Headline:  "Give a length budget",
			Technique: "Length corrections happen when the model returns more (or less) than you expected. A length budget — 'one function', 'two sentences', 'under 20 lines' — sets a concrete ceiling and cuts overshoot before it starts.",
			WeakEx:    "Write a helper to parse ISO dates",
			StrongEx:  "Write a Go helper function to parse ISO 8601 dates.\nUnder 15 lines. No error wrapping, just return the zero time on failure.",
		},
	},
	"correction_intent_warn": {
		{
			Metric:    "correction_rate",
			SubMetric: "intent",
			Level:     "warn",
			Headline:  "Lead with the task, not the context",
			Technique: "Intent corrections happen when the model misreads what you want because the context came first and buried the actual ask. Flip the structure: state the goal in the first sentence, then provide supporting context. 'Goal: X. Context: Y.' is almost never misread.",
			WeakEx:    "I've been working on a parser and the tests keep failing because of how nil is handled. Can you help?",
			StrongEx:  "Fix the nil pointer dereference in ParseFile at line 43.\nContext: the tests fail when input is an empty string; the nil check at line 38 is not reached.",
		},
		{
			Metric:    "correction_rate",
			SubMetric: "intent",
			Level:     "warn",
			Headline:  "State the goal, not just the action",
			Technique: "Describing the action ('add logging') without the goal ('so I can trace request flow') leaves the model free to implement it in ways you don't expect. One goal sentence — 'so that X' — anchors the implementation and prevents the most common class of intent mismatch.",
			WeakEx:    "Add logging to the HTTP handler",
			StrongEx:  "Add request/response logging to the HTTP handler so I can trace latency per endpoint.\nLog to stderr. Use log/slog. Do not log request bodies.",
		},
	},
	"correction_intent_ok": {
		{
			Metric:    "correction_rate",
			SubMetric: "intent",
			Level:     "ok",
			Headline:  "Anchor with an example",
			Technique: "When intent is subtle, a concrete example disambiguates faster than any description. 'Like X, but for Y' or 'Output should look like: [example]' eliminates the gap between what you mean and what the model infers.",
			WeakEx:    "Reformat the output to be more readable",
			StrongEx:  "Reformat the output to match this structure:\n  Model     Tokens    Cost\n  ------    ------    ----\n  sonnet    1.2M      $1.20\nAlign columns, right-justify numbers.",
		},
		{
			Metric:    "correction_rate",
			SubMetric: "intent",
			Level:     "ok",
			Headline:  "Use 'I want X, not Y'",
			Technique: "The 'X, not Y' pattern is the most efficient way to pre-empt an intent mismatch. It takes one extra clause and rules out the exact wrong answer before the model can produce it.",
			WeakEx:    "Explain the caching strategy",
			StrongEx:  "Explain the caching strategy — I want a conceptual overview, not implementation details or code.",
		},
	},
}

// SelectCoachingTips returns one tip per detected correction type when
// correction_rate is the weakest metric, or a single tip for other metrics.
// Tip selection is randomised so it changes on each call.
// Returns nil when all metrics are good or data is insufficient.
func SelectCoachingTips(r *ClarityReport) []*CoachingTip {
	if r == nil || r.SessionCount < 2 {
		return nil
	}

	o := r.Overall

	ci := CorrectionRateInsight(o.CorrectionRate)
	cli := ClarificationRateInsight(o.ClarificationRate)
	fi := FrontLoadRatioInsight(o.FrontLoadRatio)
	if ci.Level == "good" && cli.Level == "good" && fi.Level == "good" {
		return nil
	}

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

	if worstLevel == "good" {
		return nil
	}

	// For correction_rate with detected types: one randomly-selected tip per type.
	if worstMetric == "correction_rate" && len(o.CorrectionsByType) > 0 {
		var result []*CoachingTip
		// Fixed order so tips appear consistently: scope → format → intent
		for _, ctype := range []string{"scope", "format", "intent"} {
			if o.CorrectionsByType[ctype] == 0 {
				continue
			}
			key := "correction_" + ctype + "_" + worstLevel
			if bucket, ok := tipBank[key]; ok && len(bucket) > 0 {
				t := bucket[rand.Intn(len(bucket))]
				result = append(result, &t)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	// Generic single tip for this metric.
	key := worstMetric + "_" + worstLevel
	bucket, ok := tipBank[key]
	if !ok || len(bucket) == 0 {
		key = worstMetric + "_warn"
		bucket = tipBank[key]
		if len(bucket) == 0 {
			return nil
		}
	}
	t := bucket[rand.Intn(len(bucket))]
	return []*CoachingTip{&t}
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

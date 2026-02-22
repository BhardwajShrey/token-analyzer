package main

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// ---- Signal lists ----

var walkbackSignals = []string{
	"no,", "no.", "no!", "no no",
	"actually", "actually,",
	"wait,", "wait.", "wait wait",
	"wrong,", "wrong.", "that's wrong", "thats wrong", "that was wrong",
	"scratch that", "ignore that", "disregard that",
	"nevermind", "never mind", "forget that", "forget it",
	"instead,", "instead.",
	"hold on", "hold on,",
	"try again", "undo that", "go back", "start over",
	"that's not right", "thats not right",
	"revert", "revert that",
}

var scopePhrases = []string{
	"don't change", "dont change",
	"don't touch", "dont touch",
	"don't modify", "dont modify",
	"don't alter", "dont alter",
	"don't update", "dont update",
	"don't add", "dont add",
	"don't remove", "dont remove",
	"don't delete", "dont delete",
	"don't refactor", "dont refactor",
	"only change", "only modify", "only fix", "only in", "only the",
	"just that one", "just this one", "just that file",
	"without changing", "without modifying",
	"preserve the", "keep it the same", "keep everything else",
	"leave it", "leave that", "leave the",
	"not the entire", "not everything",
	"that file only", "this file only",
}

var formatPhrases = []string{
	"as code", "in code", "not prose",
	"as a function", "as a class", "as a method", "as a snippet",
	"in json", "in yaml", "in xml", "in markdown",
	"in typescript", "in python", "in go ", "in rust", "in javascript",
	"as plain text", "as text",
	"shorter", "simpler", "more concise", "less verbose",
	"just the code", "code only",
	"without explanation", "without comments", "no comments",
	"don't explain", "dont explain", "no prose",
	"as bullet", "as bullets", "in bullet", "as a list",
	"in one line", "as one", "as a single",
	"as markdown", "as a table", "in pseudocode",
	"format it as", "formatted as", "format as",
	"more readable",
}

var intentSignals = []string{
	"that's not", "thats not",
	"not quite", "not exactly",
	"not what i", "not what i asked", "not what i want",
	"not what i'm looking", "not looking for",
	"i meant", "what i meant",
	"i was asking", "i'm asking about",
	"you misunderstood", "misunderstood",
	"you missed", "you missed the point",
	"you got it wrong", "you're off track", "youre off track",
	"that's incorrect", "thats incorrect",
	"that's the wrong", "thats the wrong",
	"that's not it", "thats not it",
	"completely wrong", "totally wrong",
	"beside the point",
	"let me rephrase", "let me clarify",
	"not right",
}

var clarificationSignals = []string{
	"could you clarify", "can you clarify", "what do you mean",
	"do you want", "which do you", "can you specify",
	"are you referring", "could you provide more",
	"what type of", "what kind of", "can you elaborate",
	"what exactly", "could you elaborate",
}

// ---- Text extraction ----

// extractText pulls plain text from message.content.
// Handles string content and []contentBlock arrays.
// Skips tool_result and tool_use blocks.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Plain string content (user messages)
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return ""
		}
		return s
	}
	// Array of content blocks
	if raw[0] == '[' {
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return ""
		}
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// isRealUserMessage returns true for genuine user prompts (not tool results).
func isRealUserMessage(rec MessageRecord) bool {
	if rec.Type != "user" {
		return false
	}
	content := rec.Message.Content
	if len(content) == 0 {
		return false
	}
	// Plain string → real user message
	if content[0] == '"' {
		return true
	}
	// Array → real only if first block is not tool_result
	if content[0] == '[' {
		var blocks []struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(content, &blocks); err != nil {
			return false
		}
		if len(blocks) == 0 {
			return false
		}
		return blocks[0].Type != "tool_result"
	}
	return false
}

func containsAny(s string, phrases []string) bool {
	for _, p := range phrases {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func detectCorrectionType(text string) (string, bool) {
	preview := strings.ToLower(text)
	if len(preview) > 200 {
		preview = preview[:200]
	}
	wb := containsAny(preview, walkbackSignals)
	if wb && containsAny(preview, scopePhrases) {
		return "scope", true
	}
	if wb && containsAny(preview, formatPhrases) {
		return "format", true
	}
	if containsAny(preview, intentSignals) {
		return "intent", true
	}
	if wb {
		return "intent", true // fallback
	}
	return "", false
}

func hasClarificationSignal(text string) bool {
	lower := strings.ToLower(text)
	for _, sig := range clarificationSignals {
		if strings.Contains(lower, sig) {
			return true
		}
	}
	return false
}

// mondayOf returns the Monday (UTC) of the week containing t.
func mondayOf(t time.Time) time.Time {
	utc := t.UTC()
	wd := utc.Weekday()
	var daysBack int
	switch wd {
	case time.Tuesday:
		daysBack = 1
	case time.Wednesday:
		daysBack = 2
	case time.Thursday:
		daysBack = 3
	case time.Friday:
		daysBack = 4
	case time.Saturday:
		daysBack = 5
	case time.Sunday:
		daysBack = 6
	default: // Monday
		daysBack = 0
	}
	return time.Date(utc.Year(), utc.Month(), utc.Day()-daysBack, 0, 0, 0, 0, time.UTC)
}

// ---- Per-session state ----

type sessionClarityState struct {
	userMessages       []string
	firstAssistantText string
	hadClarification   bool
	correctionCount    int
	correctionCounts   map[string]int // "scope"->N, "format"->N, "intent"->N
	startTime          time.Time
}

// ---- Main computation ----

// ComputeClarity processes session JSONL files to produce a ClarityReport.
// cutoff is the oldest allowed record timestamp; zero means no cutoff.
func ComputeClarity(files []FileInfo, cutoff time.Time) *ClarityReport {
	stateMap := make(map[string]*sessionClarityState)

	for _, fi := range files {
		if fi.Kind != KindSession {
			continue
		}

		records, _ := ParseFileAllRecords(fi.Path)

		for _, rec := range records {
			// Apply date cutoff
			if !cutoff.IsZero() && !rec.Timestamp.IsZero() && rec.Timestamp.Before(cutoff) {
				continue
			}

			sessionID := rec.SessionID
			if sessionID == "" {
				continue
			}

			state, ok := stateMap[sessionID]
			if !ok {
				state = &sessionClarityState{correctionCounts: make(map[string]int)}
				stateMap[sessionID] = state
			}

			// Track earliest timestamp as session start
			if !rec.Timestamp.IsZero() && state.startTime.IsZero() {
				state.startTime = rec.Timestamp
			}

			if isRealUserMessage(rec) {
				text := extractText(rec.Message.Content)
				if text != "" {
					if len(state.userMessages) >= 1 {
						if ctype, ok := detectCorrectionType(text); ok {
							state.correctionCounts[ctype]++
							state.correctionCount++
						}
					}
					state.userMessages = append(state.userMessages, text)
				}
			}

			if rec.Type == "assistant" && state.firstAssistantText == "" {
				text := extractText(rec.Message.Content)
				if text != "" {
					state.firstAssistantText = text
					state.hadClarification = hasClarificationSignal(text)
				}
			}
		}
	}

	// Per-session metrics
	type sessionMetrics struct {
		corrRate          float64
		clarRate          float64
		frontLoad         float64
		score             float64
		startTime         time.Time
		correctionsByType map[string]float64
	}

	var allMetrics []sessionMetrics

	for _, state := range stateMap {
		userMsgCount := len(state.userMessages)
		if userMsgCount == 0 {
			continue // skip tool-only sessions
		}

		denom := userMsgCount - 1
		if denom < 1 {
			denom = 1
		}
		corrRate := float64(state.correctionCount) / float64(denom)
		if corrRate > 1 {
			corrRate = 1
		}

		var frontLoad float64
		totalLen := 0
		for _, m := range state.userMessages {
			totalLen += len(m)
		}
		if totalLen > 0 {
			frontLoad = float64(len(state.userMessages[0])) / float64(totalLen)
		}

		var clarRate float64
		if state.hadClarification {
			clarRate = 1.0
		}

		score := 100 * (0.40*frontLoad + 0.35*(1-corrRate) + 0.25*(1-clarRate))

		correctionsByType := make(map[string]float64)
		for ctype, count := range state.correctionCounts {
			correctionsByType[ctype] = float64(count) / float64(denom)
		}

		allMetrics = append(allMetrics, sessionMetrics{
			corrRate:          corrRate,
			clarRate:          clarRate,
			frontLoad:         frontLoad,
			score:             score,
			startTime:         state.startTime,
			correctionsByType: correctionsByType,
		})
	}

	sessionCount := len(allMetrics)
	if sessionCount < 2 {
		return &ClarityReport{SessionCount: sessionCount}
	}

	// Overall: mean across sessions
	var sumCorr, sumClar, sumFront, sumScore float64
	n := float64(sessionCount)
	typeSums := map[string]float64{}
	for _, m := range allMetrics {
		sumCorr += m.corrRate
		sumClar += m.clarRate
		sumFront += m.frontLoad
		sumScore += m.score
		for ctype, rate := range m.correctionsByType {
			typeSums[ctype] += rate
		}
	}
	overall := ClarityMetrics{
		CorrectionRate:    sumCorr / n,
		ClarificationRate: sumClar / n,
		FrontLoadRatio:    sumFront / n,
		Score:             sumScore / n,
	}
	overall.CorrectionsByType = make(map[string]float64)
	for ctype, sum := range typeSums {
		overall.CorrectionsByType[ctype] = sum / n
	}

	// Weekly grouping
	type weekAccum struct {
		corrSum   float64
		clarSum   float64
		frontSum  float64
		scoreSum  float64
		count     int
	}
	weekMap := make(map[string]*weekAccum)

	for _, m := range allMetrics {
		if m.startTime.IsZero() {
			continue
		}
		weekKey := mondayOf(m.startTime).Format("2006-01-02")
		wa, ok := weekMap[weekKey]
		if !ok {
			wa = &weekAccum{}
			weekMap[weekKey] = wa
		}
		wa.corrSum += m.corrRate
		wa.clarSum += m.clarRate
		wa.frontSum += m.frontLoad
		wa.scoreSum += m.score
		wa.count++
	}

	var weekly []WeeklyClarity
	for weekKey, wa := range weekMap {
		if wa.count == 0 {
			continue
		}
		cnt := float64(wa.count)
		weekly = append(weekly, WeeklyClarity{
			WeekStart:         weekKey,
			CorrectionRate:    wa.corrSum / cnt,
			ClarificationRate: wa.clarSum / cnt,
			FrontLoadRatio:    wa.frontSum / cnt,
			Score:             wa.scoreSum / cnt,
			SessionCount:      wa.count,
		})
	}
	sort.Slice(weekly, func(i, j int) bool {
		return weekly[i].WeekStart < weekly[j].WeekStart
	})

	// Hourly grouping (local time)
	type hourAccum struct {
		scoreSum float64
		count    int
	}
	var hourMap [24]hourAccum
	for _, m := range allMetrics {
		if m.startTime.IsZero() {
			continue
		}
		h := m.startTime.Local().Hour()
		hourMap[h].scoreSum += m.score
		hourMap[h].count++
	}

	hourlyBuckets := make([]HourlyClarityBucket, 24)
	bestHour, worstHour := -1, -1
	bestScore, worstScore := -1.0, 101.0
	for h := 0; h < 24; h++ {
		hourlyBuckets[h] = HourlyClarityBucket{Hour: h, Score: -1}
		if hourMap[h].count > 0 {
			avg := hourMap[h].scoreSum / float64(hourMap[h].count)
			hourlyBuckets[h].Score = avg
			hourlyBuckets[h].SessionCount = hourMap[h].count
			if avg > bestScore {
				bestScore = avg
				bestHour = h
			}
			if avg < worstScore {
				worstScore = avg
				worstHour = h
			}
		}
	}
	// Only expose best/worst if at least 2 distinct hours have data
	hoursWithData := 0
	for _, b := range hourlyBuckets {
		if b.Score >= 0 {
			hoursWithData++
		}
	}
	if hoursWithData < 2 {
		bestHour, worstHour = -1, -1
	}

	result := &ClarityReport{
		Overall:       overall,
		Weekly:        weekly,
		SessionCount:  sessionCount,
		HourlyBuckets: hourlyBuckets,
		BestHour:      bestHour,
		WorstHour:     worstHour,
	}
	result.Tips = SelectCoachingTips(result)
	result.ScoreDelta = computeWeekDelta(result.Weekly)
	return result
}

// ---- Insight functions ----

// MetricInsight carries a level and a one-line explanation.
type MetricInsight struct {
	Level    string // "good", "ok", "warn"
	Oneliner string
}

func CorrectionRateInsight(r float64) MetricInsight {
	switch {
	case r < 0.10:
		return MetricInsight{"good", "Few walk-backs — your prompts are landing first try."}
	case r < 0.25:
		return MetricInsight{"ok", "Moderate. Add a constraints block and name the output format upfront."}
	default:
		return MetricInsight{"warn", "High. Specify scope, output format, and constraints before writing the request."}
	}
}

func ClarificationRateInsight(r float64) MetricInsight {
	switch {
	case r < 0.15:
		return MetricInsight{"good", "Model rarely needs more info — prompts are clear."}
	case r < 0.35:
		return MetricInsight{"ok", "Occasional ambiguity. Add output format and scope upfront."}
	default:
		return MetricInsight{"warn", "Model asks frequently. Include what you want and what you don't."}
	}
}

func FrontLoadRatioInsight(r float64) MetricInsight {
	switch {
	case r > 0.60:
		return MetricInsight{"good", "Strong front-loading — context is established upfront."}
	case r > 0.40:
		return MetricInsight{"ok", "Moderate. Push more context into your first message."}
	default:
		return MetricInsight{"warn", "Paste all relevant code, constraints, and context into your first message."}
	}
}

func ClarityScoreInsight(s float64) MetricInsight {
	switch {
	case s > 75:
		return MetricInsight{"good", "Strong prompting discipline."}
	case s > 50:
		return MetricInsight{"ok", "Apply the coaching tip for your weakest metric below."}
	default:
		return MetricInsight{"warn", "Front-load constraints and context — most detail is leaking through follow-ups."}
	}
}

// CorrectionTypeHints is the short, actionable one-liner shown next to each
// correction type in the terminal breakdown tree.
var CorrectionTypeHints = map[string]string{
	"scope":  "add a constraints block",
	"format": "name the output medium first",
	"intent": "lead with the goal, not the context",
}

// MetricDescriptions provides tooltip/description text for each metric.
var MetricDescriptions = map[string]string{
	"total_tokens":        "Sum of all token types: input, output, cache writes, and cache reads.",
	"cache_efficiency":    "Cache reads ÷ (input + cache writes + cache reads). Higher means cheaper — cached tokens cost ~10% of fresh input.",
	"estimated_cost":      "Estimated USD based on Anthropic's per-model pricing. Cache reads are billed at a discount.",
	"sessions":            "Number of Claude Code conversation sessions across all projects.",
	"input_tokens":        "Uncached prompt tokens — the portion of your context not served from cache.",
	"output_tokens":       "Tokens generated by the model. Output is billed at 5× the input rate.",
	"correction_rate":     "% of your messages that walk back or contradict a prior request. Measures how precisely you specified intent the first time.",
	"clarification_rate":  "% of sessions where the model asked a clarifying question in its first response. High = your prompts are underspecified.",
	"front_load_ratio":    "% of your total prompt text that was in your first message. High = you front-loaded context; low = you trickled it in reactively.",
	"clarity_score":       "Composite 0–100 from the three clarity signals. Tracks your prompting discipline over time.",
}

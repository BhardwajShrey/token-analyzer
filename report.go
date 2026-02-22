package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorGray    = "\033[90m"
)

// isTerminal returns true if w is a real TTY.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Printer wraps output and applies colors only when useColors is true.
type Printer struct {
	w         io.Writer
	useColors bool
}

func (p *Printer) color(code, s string) string {
	if !p.useColors {
		return s
	}
	return code + s + colorReset
}

func (p *Printer) bold(s string) string    { return p.color(colorBold, s) }
func (p *Printer) dim(s string) string     { return p.color(colorDim, s) }
func (p *Printer) green(s string) string   { return p.color(colorGreen, s) }
func (p *Printer) yellow(s string) string  { return p.color(colorYellow, s) }
func (p *Printer) red(s string) string     { return p.color(colorRed, s) }
func (p *Printer) cyan(s string) string    { return p.color(colorCyan, s) }
func (p *Printer) magenta(s string) string { return p.color(colorMagenta, s) }
func (p *Printer) gray(s string) string    { return p.color(colorGray, s) }

func (p *Printer) printf(format string, args ...any) {
	fmt.Fprintf(p.w, format, args...)
}

func (p *Printer) println(s string) {
	fmt.Fprintln(p.w, s)
}

// ---- Formatting helpers ----

func fmtTokens(n int64) string {
	if n == 0 {
		return "0"
	}
	// Insert commas
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func fmtPct(f float64) string {
	return fmt.Sprintf("%.1f%%", f*100)
}

func fmtCost(f float64) string {
	if f < 0.01 && f > 0 {
		return fmt.Sprintf("$%.4f", f)
	}
	return fmt.Sprintf("$%.2f", f)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("Jan 02 15:04")
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("Jan 02, 2006")
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}

func fmtHourOfDay(h int) string {
	switch {
	case h == 0:
		return "12am"
	case h < 12:
		return fmt.Sprintf("%dam", h)
	case h == 12:
		return "12pm"
	default:
		return fmt.Sprintf("%dpm", h-12)
	}
}

func shortSession(id string) string {
	if len(id) >= 8 {
		return id[:8] + "…"
	}
	return id
}

// ---- Sparkline ----

var sparkChars = []rune{'░', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func sparkline(values []int64) string {
	if len(values) == 0 {
		return ""
	}
	var maxVal int64
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	var sb strings.Builder
	for _, v := range values {
		if v == 0 {
			sb.WriteRune(sparkChars[0])
		} else {
			idx := int(math.Round(float64(v)/float64(maxVal)*float64(len(sparkChars)-2))) + 1
			if idx >= len(sparkChars) {
				idx = len(sparkChars) - 1
			}
			sb.WriteRune(sparkChars[idx])
		}
	}
	return sb.String()
}

// ---- Cache efficiency bar ----

func cacheBar(pct float64, width int) string {
	filled := int(math.Round(pct * float64(width)))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// ---- Section divider ----

func sectionHeader(p *Printer, title string) {
	line := "── " + title + " " + strings.Repeat("─", max(0, 54-len(title)))
	p.println(p.bold(line))
	p.println("")
}

// ---- Main report printer ----

func PrintReport(w io.Writer, r *AggregatedReport, useColors bool) {
	p := &Printer{w: w, useColors: useColors}

	// Header
	p.println(p.bold("╔══════════════════════════════════════════════════════╗"))
	p.println(p.bold("║          CLAUDE CODE TOKEN ANALYZER                  ║"))
	period := periodStr(r)
	padded := fmt.Sprintf("%-52s", "║  Period: "+period)
	p.println(p.bold(padded + "║"))
	p.println(p.bold("╚══════════════════════════════════════════════════════╝"))
	p.println("")

	printOverallSummary(p, r)
	printModelBreakdown(p, r)
	printProjects(p, r)
	printSessions(p, r)
	printDailyTrend(p, r)
	printInsights(p, r)
	printClaritySection(p, r)
	printCoachingSection(p, r)
}

func periodStr(r *AggregatedReport) string {
	if r.FilterDays > 0 {
		return fmt.Sprintf("Last %d days", r.FilterDays)
	}
	if r.DateFrom.IsZero() {
		return "No data"
	}
	return fmtDate(r.DateFrom) + " – " + fmtDate(r.DateTo)
}

func printOverallSummary(p *Printer, r *AggregatedReport) {
	sectionHeader(p, "OVERALL SUMMARY")

	total := r.Grand.TotalTokens()

	pctOf := func(n int64) string {
		if total == 0 {
			return "0%"
		}
		return fmtPct(float64(n) / float64(total))
	}

	p.printf("  %-28s  %14s  %8s\n",
		"Input tokens", fmtTokens(r.Grand.InputTokens), p.gray("("+pctOf(r.Grand.InputTokens)+")"))
	p.printf("  %-28s  %14s  %8s\n",
		"Output tokens", fmtTokens(r.Grand.OutputTokens), p.gray("("+pctOf(r.Grand.OutputTokens)+")"))
	p.printf("  %-28s  %14s  %8s\n",
		"Cache writes", fmtTokens(r.Grand.CacheCreationInputTokens), p.gray("("+pctOf(r.Grand.CacheCreationInputTokens)+")"))
	p.printf("  %-28s  %14s  %8s\n",
		"Cache reads", fmtTokens(r.Grand.CacheReadInputTokens), p.gray("("+pctOf(r.Grand.CacheReadInputTokens)+")"))
	p.println("  " + strings.Repeat("─", 54))
	p.printf("  %-28s  %14s\n", p.bold("Total tokens"), p.bold(fmtTokens(total)))
	p.println("")

	eff := r.Grand.CacheEfficiency()
	bar := cacheBar(eff, 20)
	effStr := fmt.Sprintf("%.1f%%  %s", eff*100, bar)
	label := "Cache efficiency"
	if eff >= 0.75 {
		effStr += "  " + p.green("excellent")
		label = p.green(label)
	} else if eff >= 0.40 {
		effStr += "  " + p.yellow("moderate")
		label = p.yellow(label)
	} else {
		effStr += "  " + p.red("low")
		label = p.red(label)
	}
	p.printf("  %-28s  %s\n", label, effStr)
	p.printf("  %-28s  %s\n", "Estimated cost", p.bold(fmtCost(r.Grand.CostUSD)))
	p.println("")

	// Session counts
	sessionCount := len(r.Sessions)
	subCount := 0
	for _, s := range r.Sessions {
		if s.SubagentTotals.TotalTokens() > 0 {
			subCount++
		}
	}
	models := len(r.ModelSummaries)
	p.printf("  %-28s  %d  %s\n", "Sessions", sessionCount, p.gray(fmt.Sprintf("(%d with subagents)", subCount)))
	p.printf("  %-28s  %d  %s\n", "Models used", models, p.gray(modelList(r.ModelSummaries)))
	p.println("")
}

func modelList(m map[string]*UsageTotals) string {
	var names []string
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	if len(names) <= 3 {
		return "(" + strings.Join(names, ", ") + ")"
	}
	return "(" + strings.Join(names[:3], ", ") + ", …)"
}

func printModelBreakdown(p *Printer, r *AggregatedReport) {
	if len(r.ModelSummaries) == 0 {
		return
	}
	sectionHeader(p, "TOKEN BREAKDOWN BY MODEL")

	// Sort models by total tokens
	type mEntry struct {
		name   string
		totals *UsageTotals
	}
	var entries []mEntry
	for k, v := range r.ModelSummaries {
		entries = append(entries, mEntry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].totals.TotalTokens() > entries[j].totals.TotalTokens()
	})

	header := fmt.Sprintf("  %-36s  %10s  %10s  %10s  %10s  %8s",
		"Model", "Input", "Output", "Cache Wr", "Cache Rd", "Cost")
	p.println(p.dim(header))
	p.println("  " + strings.Repeat("─", 92))

	for _, e := range entries {
		p.printf("  %-36s  %10s  %10s  %10s  %10s  %8s\n",
			truncate(e.name, 36),
			fmtTokens(e.totals.InputTokens),
			fmtTokens(e.totals.OutputTokens),
			fmtTokens(e.totals.CacheCreationInputTokens),
			fmtTokens(e.totals.CacheReadInputTokens),
			fmtCost(e.totals.CostUSD),
		)
	}
	p.println("")
}

func printProjects(p *Printer, r *AggregatedReport) {
	if len(r.Projects) == 0 {
		return
	}
	sectionHeader(p, "PROJECTS BY TOKEN USAGE")

	header := fmt.Sprintf("  %-3s  %-24s  %14s  %10s  %8s  %8s",
		"#", "Project", "Total Tokens", "Cache Eff.", "Cost", "Sessions")
	p.println(p.dim(header))
	p.println("  " + strings.Repeat("─", 78))

	for i, proj := range r.Projects {
		eff := proj.Totals.CacheEfficiency()
		effFmt := fmtPct(eff)
		if eff >= 0.75 {
			effFmt = p.green(effFmt)
		} else if eff >= 0.40 {
			effFmt = p.yellow(effFmt)
		} else {
			effFmt = p.red(effFmt)
		}
		p.printf("  %-3d  %-24s  %14s  %10s  %8s  %8d\n",
			i+1,
			truncate(proj.Name, 24),
			fmtTokens(proj.Totals.TotalTokens()),
			effFmt,
			fmtCost(proj.Totals.CostUSD),
			proj.SessionCount,
		)
		p.println(p.gray("       " + truncate(proj.Path, 70)))
	}
	p.println("")
}

func printSessions(p *Printer, r *AggregatedReport) {
	if len(r.Sessions) == 0 {
		return
	}
	sectionHeader(p, "TOP SESSIONS")

	limit := 10
	if len(r.Sessions) < limit {
		limit = len(r.Sessions)
	}

	header := fmt.Sprintf("  %-3s  %-12s  %-18s  %-14s  %12s  %12s  %8s",
		"#", "Session", "Project", "Started", "Tokens", "Subagent", "Cost")
	p.println(p.dim(header))
	p.println("  " + strings.Repeat("─", 92))

	for i, sess := range r.Sessions[:limit] {
		combined := fmtTokens(sess.Totals.TotalTokens())
		subStr := "—"
		if sess.SubagentTotals.TotalTokens() > 0 {
			subStr = fmtTokens(sess.SubagentTotals.TotalTokens())
		}
		p.printf("  %-3d  %-12s  %-18s  %-14s  %12s  %12s  %8s\n",
			i+1,
			shortSession(sess.SessionID),
			truncate(sess.ProjectName, 18),
			fmtTime(sess.StartTime),
			combined,
			subStr,
			fmtCost(sess.Totals.CostUSD+sess.SubagentTotals.CostUSD),
		)
	}
	if len(r.Sessions) > limit {
		p.println(p.gray(fmt.Sprintf("  … and %d more sessions", len(r.Sessions)-limit)))
	}
	p.println("")
}

func printDailyTrend(p *Printer, r *AggregatedReport) {
	if len(r.Daily) == 0 {
		return
	}
	sectionHeader(p, "DAILY TOKEN TREND")

	// Extract daily totals for sparkline
	vals := make([]int64, len(r.Daily))
	var maxVal int64
	for i, d := range r.Daily {
		vals[i] = d.Totals.TotalTokens()
		if vals[i] > maxVal {
			maxVal = vals[i]
		}
	}

	spark := sparkline(vals)
	runes := []rune(spark)

	for i, d := range r.Daily {
		var bar string
		if i < len(runes) {
			bar = string(runes[i])
		}
		tokens := d.Totals.TotalTokens()

		var tokenFmt string
		if tokens == 0 {
			tokenFmt = p.gray("0")
		} else {
			tokenFmt = fmtTokens(tokens)
		}

		// Print individual bar for each day using block chars scaled to 20 width
		barWidth := 20
		var dayBar string
		if tokens == 0 {
			dayBar = p.gray(strings.Repeat("░", barWidth))
		} else {
			filled := int(math.Round(float64(tokens) / float64(maxVal) * float64(barWidth)))
			if filled == 0 {
				filled = 1
			}
			dayBar = p.cyan(strings.Repeat("█", filled)) + p.gray(strings.Repeat("░", barWidth-filled))
		}

		_ = bar // sparkline char used for reference
		p.printf("  %s  %s  %s\n", d.Date, dayBar, tokenFmt)
	}
	p.println("")
}

func printInsights(p *Printer, r *AggregatedReport) {
	if len(r.Insights) == 0 {
		return
	}
	sectionHeader(p, "INSIGHTS")

	for _, ins := range r.Insights {
		var tag string
		var msgFmt func(string) string
		switch ins.Severity {
		case "good":
			tag = p.green("[GOOD]")
			msgFmt = p.green
		case "warn":
			tag = p.yellow("[WARN]")
			msgFmt = p.yellow
		default:
			tag = p.cyan("[INFO]")
			msgFmt = func(s string) string { return s }
		}
		// Word-wrap at ~70 chars
		wrapped := wordWrap(ins.Message, 68)
		lines := strings.Split(wrapped, "\n")
		p.printf("  %s  %s\n", tag, msgFmt(lines[0]))
		for _, line := range lines[1:] {
			p.printf("         %s\n", msgFmt(line))
		}
		p.println("")
	}
}

// wordWrap wraps s at width characters, breaking at spaces.
func wordWrap(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	var sb strings.Builder
	lineLen := 0
	for i, w := range words {
		wLen := utf8.RuneCountInString(w)
		if i > 0 && lineLen+1+wLen > width {
			sb.WriteByte('\n')
			lineLen = 0
		} else if i > 0 {
			sb.WriteByte(' ')
			lineLen++
		}
		sb.WriteString(w)
		lineLen += wLen
	}
	return sb.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---- Prompt Clarity section ----

func printClaritySection(p *Printer, r *AggregatedReport) {
	sectionHeader(p, "PROMPT CLARITY")

	if r.Clarity == nil || r.Clarity.SessionCount < 2 {
		p.println("  Not enough data yet (need 2+ sessions)")
		p.println("")
		return
	}

	cl := r.Clarity

	// Clarity Score row
	score := cl.Overall.Score
	si := ClarityScoreInsight(score)
	bar := cacheBar(score/100, 20)
	var scoreBadge, coloredBar string
	switch si.Level {
	case "good":
		scoreBadge = p.green("[good]")
		coloredBar = p.green(bar)
	case "ok":
		scoreBadge = p.yellow("[ok]")
		coloredBar = p.yellow(bar)
	default:
		scoreBadge = p.red("[warn]")
		coloredBar = p.red(bar)
	}
	p.printf("  %-22s  %d/100  %s  %s\n", "Clarity Score", int(math.Round(score)), coloredBar, scoreBadge)
	p.printf("  %-22s  %s\n", "", p.dim(`"`+si.Oneliner+`"`))
	p.println("")

	// Weekly trend sparkline
	if len(cl.Weekly) > 1 {
		var wVals []int64
		for _, w := range cl.Weekly {
			wVals = append(wVals, int64(math.Round(w.Score)))
		}
		spark := sparkline(wVals)
		runes := []rune(spark)

		var trendStr string
		first := cl.Weekly[0].Score
		last := cl.Weekly[len(cl.Weekly)-1].Score
		switch {
		case last > first+2:
			trendStr = p.green("(↑ improving)")
		case last < first-2:
			trendStr = p.red("(↓ declining)")
		default:
			trendStr = p.gray("(→ stable)")
		}

		var sb strings.Builder
		for i := range cl.Weekly {
			if i > 0 {
				sb.WriteString("  ")
			}
			ch := rune('░')
			if i < len(runes) {
				ch = runes[i]
			}
			fmt.Fprintf(&sb, "W%d%c", i+1, ch)
		}
		p.printf("  %-22s  %s  %s\n", "Weekly trend", sb.String(), trendStr)
		p.println("")
	}

	// Time-of-day row
	if cl.BestHour >= 0 {
		bestLabel := fmtHourOfDay(cl.BestHour)
		worstLabel := fmtHourOfDay(cl.WorstHour)
		bestScoreStr := fmt.Sprintf("%d", int(math.Round(cl.HourlyBuckets[cl.BestHour].Score)))
		worstScoreStr := fmt.Sprintf("%d", int(math.Round(cl.HourlyBuckets[cl.WorstHour].Score)))
		p.printf("  %-22s  %s · %s\n",
			"Time-of-day",
			p.green("Sharpest "+bestLabel+" ("+bestScoreStr+")"),
			p.red("Sloppiest "+worstLabel+" ("+worstScoreStr+")"),
		)
		p.println("")
	}

	// Individual metric rows
	printClarityMetricRow(p, "Correction Rate", cl.Overall.CorrectionRate, "↓ lower is better",
		CorrectionRateInsight(cl.Overall.CorrectionRate), MetricDescriptions["correction_rate"],
		cl.Overall.CorrectionsByType)
	printClarityMetricRow(p, "Clarification Rate", cl.Overall.ClarificationRate, "↓ lower is better",
		ClarificationRateInsight(cl.Overall.ClarificationRate), MetricDescriptions["clarification_rate"],
		nil)
	printClarityMetricRow(p, "Front-load Ratio", cl.Overall.FrontLoadRatio, "↑ higher is better",
		FrontLoadRatioInsight(cl.Overall.FrontLoadRatio), MetricDescriptions["front_load_ratio"],
		nil)
}

// ---- Coaching tip section ----

var metricDisplayNames = map[string]string{
	"correction_rate":    "Correction Rate",
	"clarification_rate": "Clarification Rate",
	"front_load_ratio":   "Front-load Ratio",
}

var subMetricDisplayNames = map[string]string{
	"scope":  "Scope corrections",
	"format": "Format corrections",
	"intent": "Intent corrections",
}

func printCoachingSection(p *Printer, r *AggregatedReport) {
	if r.Clarity == nil || len(r.Clarity.Tips) == 0 {
		return
	}

	cl := r.Clarity
	sectionHeader(p, "COACHING TIP")

	for i, tip := range cl.Tips {
		if i > 0 {
			p.println("  " + p.gray(strings.Repeat("·", 54)))
			p.println("")
		}
		printOneTip(p, tip, cl, i == 0)
	}
}

func printOneTip(p *Printer, tip *CoachingTip, cl *ClarityReport, showDelta bool) {
	displayName := metricDisplayNames[tip.Metric]
	var metricVal float64
	switch tip.Metric {
	case "correction_rate":
		metricVal = cl.Overall.CorrectionRate
	case "clarification_rate":
		metricVal = cl.Overall.ClarificationRate
	case "front_load_ratio":
		metricVal = cl.Overall.FrontLoadRatio
	}
	if tip.SubMetric != "" {
		if sname, ok := subMetricDisplayNames[tip.SubMetric]; ok {
			displayName = sname
		}
		if cl.Overall.CorrectionsByType != nil {
			metricVal = cl.Overall.CorrectionsByType[tip.SubMetric]
		}
	}

	var badge string
	if tip.Level == "warn" {
		badge = p.red("[warn]")
	} else {
		badge = p.yellow("[ok]")
	}

	var deltaStr string
	if showDelta && cl.ScoreDelta != nil {
		d := *cl.ScoreDelta
		switch {
		case d > 0.5:
			deltaStr = "  " + p.green(fmt.Sprintf("↑ +%.0f pts from last week", d))
		case d < -0.5:
			deltaStr = "  " + p.red(fmt.Sprintf("↓ %.0f pts from last week", d))
		}
	}

	p.printf("  Focus: %-22s  %5.1f%%  %s%s\n", displayName, metricVal*100, badge, deltaStr)
	p.println("")

	p.printf("  %s\n", p.bold(tip.Headline))
	p.printf("  %s\n", p.dim(strings.Repeat("─", len(tip.Headline))))

	wrapped := wordWrap(tip.Technique, 68)
	for _, line := range strings.Split(wrapped, "\n") {
		p.printf("  %s\n", line)
	}
	p.println("")

	weakLines := strings.Split(tip.WeakEx, "\n")
	p.printf("  %s  %s\n", p.red("✗"), p.dim(weakLines[0]))
	for _, l := range weakLines[1:] {
		p.printf("     %s\n", p.dim(l))
	}
	p.println("")

	strongLines := strings.Split(tip.StrongEx, "\n")
	p.printf("  %s  %s\n", p.green("✓"), strongLines[0])
	for _, l := range strongLines[1:] {
		p.printf("     %s\n", l)
	}
	p.println("")
}

func printClarityMetricRow(p *Printer, name string, val float64, direction string, ins MetricInsight, description string, subBreakdown map[string]float64) {
	var badge string
	switch ins.Level {
	case "good":
		badge = p.green("[good]")
	case "ok":
		badge = p.yellow("[ok]")
	default:
		badge = p.red("[warn]")
	}
	p.printf("  %-22s  %5.1f%%  %s  %s\n", name, val*100, p.gray(direction), badge)
	p.printf("    %s\n", p.dim(`"`+ins.Oneliner+`"`))
	p.printf("    %s\n", p.gray(description))

	// Type breakdown tree
	if len(subBreakdown) > 0 {
		type entry struct {
			name string
			rate float64
		}
		var entries []entry
		for ctype, rate := range subBreakdown {
			if rate > 0 {
				entries = append(entries, entry{ctype, rate})
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].rate > entries[j].rate
		})
		for i, e := range entries {
			prefix := "├─"
			if i == len(entries)-1 {
				prefix = "└─"
			}
			label := strings.ToUpper(e.name[:1]) + e.name[1:]
			hint := CorrectionTypeHints[e.name]
			p.printf("    %s %-10s %5.1f%%  %s\n", prefix, label, e.rate*100, p.gray("→ "+hint))
		}
	}

	p.println("")
}

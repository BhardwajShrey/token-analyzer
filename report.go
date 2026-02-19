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

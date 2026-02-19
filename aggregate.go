package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// AggregateOptions controls filtering applied before aggregation.
type AggregateOptions struct {
	Days       int    // 0 = all time
	Project    string // empty = all projects
	StatsCache *StatsCache
}

// Aggregate parses all discovered files and builds the full report.
func Aggregate(files []FileInfo, opts AggregateOptions) *AggregatedReport {
	report := &AggregatedReport{
		ModelSummaries: make(map[string]*UsageTotals),
		FilterDays:     opts.Days,
		FilterProject:  opts.Project,
		PeakHour:       -1,
	}

	var cutoff time.Time
	if opts.Days > 0 {
		cutoff = time.Now().UTC().AddDate(0, 0, -opts.Days)
	}

	// Per-slug and per-session accumulators
	projectMap := make(map[string]*ProjectSummary)
	sessionMap := make(map[string]*SessionSummary)
	dailyMap := make(map[string]*UsageTotals)
	// Track cwd per slug (derived from first record with non-empty cwd)
	slugCWD := make(map[string]string)

	for _, fi := range files {
		// Apply project filter
		if opts.Project != "" {
			slug := fi.ProjectSlug
			cwd := slugCWD[slug]
			if cwd == "" {
				cwd = slugToPath(slug)
			}
			projectName := filepath.Base(cwd)
			if !containsCI(slug, opts.Project) && !containsCI(projectName, opts.Project) {
				// We'll re-check after we have cwd — skip for now if no match
				// (we may miss some; a second pass is not worth the complexity)
			}
		}

		records, errs := ParseFile(fi.Path)
		report.ParseErrors += errs

		for i, rec := range records {
			// Capture cwd from first record
			if rec.CWD != "" && slugCWD[fi.ProjectSlug] == "" {
				slugCWD[fi.ProjectSlug] = rec.CWD
			}
			// Apply project filter using cwd
			if opts.Project != "" && i == 0 {
				cwd := slugCWD[fi.ProjectSlug]
				name := filepath.Base(cwd)
				if !containsCI(fi.ProjectSlug, opts.Project) && !containsCI(name, opts.Project) {
					break // skip all records in this file
				}
			}

			// Apply date filter
			if opts.Days > 0 && rec.Timestamp.Before(cutoff) {
				continue
			}

			model := rec.Message.Model
			usage := rec.Message.Usage
			cost := ComputeCost(model, usage)

			// Update date range
			if report.DateFrom.IsZero() || rec.Timestamp.Before(report.DateFrom) {
				report.DateFrom = rec.Timestamp
			}
			if rec.Timestamp.After(report.DateTo) {
				report.DateTo = rec.Timestamp
			}

			// Grand total
			report.Grand.Add(usage, cost)

			// Per-model
			if _, ok := report.ModelSummaries[model]; !ok {
				report.ModelSummaries[model] = &UsageTotals{}
			}
			report.ModelSummaries[model].Add(usage, cost)

			// Per-project
			proj := getOrCreateProject(projectMap, fi.ProjectSlug)
			proj.Totals.Add(usage, cost)
			if _, ok := proj.ModelBreakdown[model]; !ok {
				proj.ModelBreakdown[model] = &UsageTotals{}
			}
			proj.ModelBreakdown[model].Add(usage, cost)

			// Per-session
			sess := getOrCreateSession(sessionMap, rec.SessionID, fi.ProjectSlug)
			if fi.Kind == KindSubagent {
				sess.SubagentTotals.Add(usage, cost)
			} else {
				sess.Totals.Add(usage, cost)
				if _, ok := sess.ModelBreakdown[model]; !ok {
					sess.ModelBreakdown[model] = &UsageTotals{}
				}
				sess.ModelBreakdown[model].Add(usage, cost)
			}
			// Track session time range
			if !rec.Timestamp.IsZero() {
				if sess.StartTime.IsZero() || rec.Timestamp.Before(sess.StartTime) {
					sess.StartTime = rec.Timestamp
				}
				if rec.Timestamp.After(sess.EndTime) {
					sess.EndTime = rec.Timestamp
				}
			}

			// Per-day
			date := rec.Timestamp.UTC().Format("2006-01-02")
			if _, ok := dailyMap[date]; !ok {
				dailyMap[date] = &UsageTotals{}
			}
			dailyMap[date].Add(usage, cost)
		}
	}

	// Enrich project metadata from cwd
	for slug, proj := range projectMap {
		cwd := slugCWD[slug]
		if cwd == "" {
			cwd = slugToPath(slug)
		}
		proj.Path = cwd
		proj.Name = filepath.Base(cwd)
	}

	// Enrich session metadata from project slugs
	for _, sess := range sessionMap {
		slug := sess.ProjectSlug
		if proj, ok := projectMap[slug]; ok {
			sess.ProjectName = proj.Name
		} else {
			sess.ProjectName = filepath.Base(slugToPath(slug))
		}
	}

	// Attach sessions to projects and count subagents
	for _, sess := range sessionMap {
		if proj, ok := projectMap[sess.ProjectSlug]; ok {
			proj.Sessions = append(proj.Sessions, sess)
			proj.SessionCount++
			if sess.SubagentTotals.TotalTokens() > 0 {
				proj.SubagentCount++
			}
		}
	}

	// Build sorted slices
	for _, p := range projectMap {
		report.Projects = append(report.Projects, p)
	}
	sort.Slice(report.Projects, func(i, j int) bool {
		return report.Projects[i].Totals.TotalTokens() > report.Projects[j].Totals.TotalTokens()
	})

	for _, s := range sessionMap {
		report.Sessions = append(report.Sessions, s)
	}
	sort.Slice(report.Sessions, func(i, j int) bool {
		return report.Sessions[i].CombinedTokens() > report.Sessions[j].CombinedTokens()
	})

	// Build daily summary slice (last N days or all)
	report.Daily = buildDailySlice(dailyMap, opts.Days)

	// Peak hour from stats-cache
	if opts.StatsCache != nil {
		report.PeakHour = peakHour(opts.StatsCache.HourCounts)
	}

	// Generate insights
	report.Insights = generateInsights(report, opts.StatsCache)

	return report
}

func getOrCreateProject(m map[string]*ProjectSummary, slug string) *ProjectSummary {
	if p, ok := m[slug]; ok {
		return p
	}
	p := &ProjectSummary{
		Slug:           slug,
		ModelBreakdown: make(map[string]*UsageTotals),
	}
	m[slug] = p
	return p
}

func getOrCreateSession(m map[string]*SessionSummary, sessionID, projectSlug string) *SessionSummary {
	if s, ok := m[sessionID]; ok {
		return s
	}
	s := &SessionSummary{
		SessionID:      sessionID,
		ProjectSlug:    projectSlug,
		ModelBreakdown: make(map[string]*UsageTotals),
	}
	m[sessionID] = s
	return s
}

func buildDailySlice(dailyMap map[string]*UsageTotals, days int) []DailySummary {
	var result []DailySummary

	if days > 0 {
		// Fill in all days in range, including zero-token days
		now := time.Now().UTC()
		for i := days - 1; i >= 0; i-- {
			date := now.AddDate(0, 0, -i).Format("2006-01-02")
			ds := DailySummary{Date: date}
			if totals, ok := dailyMap[date]; ok {
				ds.Totals = *totals
			}
			result = append(result, ds)
		}
	} else {
		for date, totals := range dailyMap {
			result = append(result, DailySummary{Date: date, Totals: *totals})
		}
		sort.Slice(result, func(i, j int) bool {
			return result[i].Date < result[j].Date
		})
		// Keep last 30 days for display if all-time
		if len(result) > 30 {
			result = result[len(result)-30:]
		}
	}

	return result
}

func peakHour(hourCounts map[string]int) int {
	if len(hourCounts) == 0 {
		return -1
	}
	best := -1
	bestCount := 0
	for k, v := range hourCounts {
		h, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		if v > bestCount {
			bestCount = v
			best = h
		}
	}
	return best
}

func generateInsights(r *AggregatedReport, sc *StatsCache) []Insight {
	var insights []Insight

	// 1. Cache efficiency
	eff := r.Grand.CacheEfficiency()
	switch {
	case eff >= 0.75:
		insights = append(insights, Insight{
			Severity: "good",
			Message:  fmt.Sprintf("Cache efficiency is excellent at %.1f%% — your long sessions and CLAUDE.md are working well.", eff*100),
		})
	case eff >= 0.40:
		insights = append(insights, Insight{
			Severity: "info",
			Message:  fmt.Sprintf("Cache efficiency is moderate at %.1f%%. Consider longer sessions and adding a CLAUDE.md to pre-establish context.", eff*100),
		})
	case r.Grand.TotalTokens() > 0:
		insights = append(insights, Insight{
			Severity: "warn",
			Message:  fmt.Sprintf("Cache efficiency is low at %.1f%%. Try longer sessions, avoid frequent restarts, and use CLAUDE.md to establish persistent context.", eff*100),
		})
	}

	// 2. Output token ratio vs total (using all token types as denominator so
	// cache-heavy sessions aren't falsely flagged as verbose).
	if total := r.Grand.TotalTokens(); total > 0 {
		outputRatio := float64(r.Grand.OutputTokens) / float64(total)
		if outputRatio > 0.30 {
			insights = append(insights, Insight{
				Severity: "warn",
				Message:  fmt.Sprintf("Output tokens are %.0f%% of total tokens — responses may be very verbose. Consider adding 'be concise' instructions to CLAUDE.md.", outputRatio*100),
			})
		}
	}

	// 3. Subagent overhead
	var subagentTotal int64
	for _, sess := range r.Sessions {
		subagentTotal += sess.SubagentTotals.TotalTokens()
	}
	if subagentTotal > 0 && r.Grand.TotalTokens() > 0 {
		overheadPct := float64(subagentTotal) / float64(r.Grand.TotalTokens()) * 100
		insights = append(insights, Insight{
			Severity: "info",
			Message:  fmt.Sprintf("Subagents consumed %.0f%% of total tokens (%s tokens). Each subagent spawns a fresh context window; cache reads in the main session keep the rest cheap.", overheadPct, fmtTokensInt(subagentTotal)),
		})
	}

	// 4. Peak hour
	if r.PeakHour >= 0 {
		insights = append(insights, Insight{
			Severity: "info",
			Message:  fmt.Sprintf("Your peak usage hour is %02d:00–%02d:00 local time.", r.PeakHour, r.PeakHour+1),
		})
	}

	// 5. Unrecognized models
	for model := range r.ModelSummaries {
		if _, ok := LookupPricing(model); !ok {
			insights = append(insights, Insight{
				Severity: "warn",
				Message:  fmt.Sprintf("Model %q is not in the pricing table — its cost is shown as $0.00. Add it to pricing.go.", model),
			})
		}
	}

	// 6. Parse errors
	if r.ParseErrors > 0 {
		insights = append(insights, Insight{
			Severity: "warn",
			Message:  fmt.Sprintf("%d JSONL line(s) could not be parsed (likely partial writes during streaming). Token counts may be slightly under-reported.", r.ParseErrors),
		})
	}

	return insights
}

// containsCI is a case-insensitive substring check.
func containsCI(s, sub string) bool {
	if sub == "" {
		return true
	}
	return len(s) >= len(sub) && func() bool {
		sLower := toLower(s)
		subLower := toLower(sub)
		for i := 0; i <= len(sLower)-len(subLower); i++ {
			if sLower[i:i+len(subLower)] == subLower {
				return true
			}
		}
		return false
	}()
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// fmtTokensInt formats tokens for use in insight messages.
func fmtTokensInt(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

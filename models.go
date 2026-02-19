package main

import (
	"encoding/json"
	"time"
)

// ---- Raw JSONL record types ----

// TokenUsage holds the raw API usage counts from a single assistant message.
// NOTE: These live at record.Message.Usage, NOT at a top-level record.usage field.
type TokenUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// IsZero returns true if no tokens were used (streaming prefix acknowledgments).
func (u TokenUsage) IsZero() bool {
	return u.InputTokens == 0 && u.OutputTokens == 0 &&
		u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0
}

// MessageBody is the nested "message" object inside a JSONL record.
type MessageBody struct {
	Model   string          `json:"model"`
	Usage   TokenUsage      `json:"usage"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// MessageRecord is a single line from any JSONL session file.
// Only records where Type == "assistant" carry usage data.
type MessageRecord struct {
	UUID        string      `json:"uuid"`
	ParentUUID  string      `json:"parentUuid"`
	Type        string      `json:"type"`
	SessionID   string      `json:"sessionId"`
	Timestamp   time.Time   `json:"timestamp"`
	CWD         string      `json:"cwd"`
	IsSidechain bool        `json:"isSidechain"`
	UserType    string      `json:"userType"`
	AgentID     string      `json:"agentId"`
	Slug        string      `json:"slug"`
	GitBranch   string      `json:"gitBranch"`
	Message     MessageBody `json:"message"`
}

// ---- File classification ----

// FileKind distinguishes session-level from subagent JSONL files.
type FileKind int

const (
	KindSession  FileKind = iota // <slug>/<uuid>.jsonl
	KindSubagent                 // <slug>/<uuid>/subagents/agent-<id>.jsonl
)

// FileInfo describes a discovered JSONL file.
type FileInfo struct {
	Path        string
	Kind        FileKind
	ProjectSlug string
	SessionID   string
	AgentID     string // empty for KindSession
}

// ---- Aggregated types ----

// UsageTotals is the canonical accumulator for any aggregation axis.
type UsageTotals struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	MessageCount             int64
	CostUSD                  float64
}

// Add merges a TokenUsage into this accumulator.
func (t *UsageTotals) Add(u TokenUsage, cost float64) {
	t.InputTokens += int64(u.InputTokens)
	t.OutputTokens += int64(u.OutputTokens)
	t.CacheCreationInputTokens += int64(u.CacheCreationInputTokens)
	t.CacheReadInputTokens += int64(u.CacheReadInputTokens)
	t.MessageCount++
	t.CostUSD += cost
}

// TotalTokens returns the sum of all token types.
func (t UsageTotals) TotalTokens() int64 {
	return t.InputTokens + t.OutputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens
}

// CacheEfficiency returns cache_read / (input + cache_write + cache_read) as [0,1].
func (t UsageTotals) CacheEfficiency() float64 {
	denom := t.InputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens
	if denom == 0 {
		return 0
	}
	return float64(t.CacheReadInputTokens) / float64(denom)
}

// ProjectSummary aggregates all token usage for one project.
type ProjectSummary struct {
	Slug           string
	Name           string
	Path           string
	Totals         UsageTotals
	SessionCount   int
	SubagentCount  int
	ModelBreakdown map[string]*UsageTotals
	Sessions       []*SessionSummary
}

// SessionSummary aggregates token usage for one session UUID.
type SessionSummary struct {
	SessionID      string
	ProjectName    string
	ProjectSlug    string
	StartTime      time.Time
	EndTime        time.Time
	Totals         UsageTotals // main conversation only
	SubagentTotals UsageTotals // tokens from subagent files for this session
	ModelBreakdown map[string]*UsageTotals
}

// CombinedTokens returns total tokens including subagents.
func (s *SessionSummary) CombinedTokens() int64 {
	return s.Totals.TotalTokens() + s.SubagentTotals.TotalTokens()
}

// DailySummary aggregates token usage for a calendar date.
type DailySummary struct {
	Date   string // "YYYY-MM-DD"
	Totals UsageTotals
}

// Insight is a single actionable observation surfaced in the report.
type Insight struct {
	Severity string // "good", "info", "warn"
	Message  string
}

// ClarityMetrics holds the aggregate prompt clarity measurements.
type ClarityMetrics struct {
	CorrectionRate    float64
	ClarificationRate float64
	FrontLoadRatio    float64
	Score             float64
}

// WeeklyClarity holds clarity metrics for one ISO week (Monday-based).
type WeeklyClarity struct {
	WeekStart         string // "YYYY-MM-DD" Monday
	CorrectionRate    float64
	ClarificationRate float64
	FrontLoadRatio    float64
	Score             float64
	SessionCount      int
}

// ClarityReport is the top-level clarity result attached to AggregatedReport.
type ClarityReport struct {
	Overall      ClarityMetrics
	Weekly       []WeeklyClarity // sorted asc by WeekStart
	SessionCount int
}

// AggregatedReport is the top-level result from the aggregation phase.
type AggregatedReport struct {
	Grand          UsageTotals
	ModelSummaries map[string]*UsageTotals
	Projects       []*ProjectSummary // sorted by TotalTokens desc
	Sessions       []*SessionSummary // sorted by CombinedTokens desc
	Daily          []DailySummary    // sorted by date asc
	ParseErrors    int
	Insights       []Insight
	DateFrom       time.Time
	DateTo         time.Time
	FilterDays     int
	FilterProject  string
	PeakHour       int // -1 if unknown
	Clarity        *ClarityReport
}

// ---- stats-cache.json types ----

// StatsCache represents the pre-aggregated summary file.
type StatsCache struct {
	ModelUsage    map[string]StatsCacheModel `json:"modelUsage"`
	HourCounts    map[string]int             `json:"hourCounts"`
	TotalSessions int                        `json:"totalSessions"`
	TotalMessages int                        `json:"totalMessages"`
	DailyActivity []StatsCacheDaily          `json:"dailyActivity"`
}

// StatsCacheModel holds per-model aggregate stats from stats-cache.json.
type StatsCacheModel struct {
	InputTokens              int64   `json:"inputTokens"`
	OutputTokens             int64   `json:"outputTokens"`
	CacheReadInputTokens     int64   `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int64   `json:"cacheCreationInputTokens"`
	CostUSD                  float64 `json:"costUSD"`
}

// StatsCacheDaily holds per-day activity counts from stats-cache.json.
type StatsCacheDaily struct {
	Date         string `json:"date"`
	MessageCount int    `json:"messageCount"`
	SessionCount int    `json:"sessionCount"`
}

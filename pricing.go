package main

import "strings"

// ModelPricing holds per-million-token rates for a model family.
type ModelPricing struct {
	Family            string
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheWritePerMTok float64
	CacheReadPerMTok  float64
}

// pricingTable maps model family prefixes to pricing.
// Longest-prefix matching is used so versioned IDs like
// "claude-sonnet-4-5-20250929" correctly match "claude-sonnet-4".
var pricingTable = []ModelPricing{
	{
		Family:            "claude-opus-4",
		InputPerMTok:      15.00,
		OutputPerMTok:     75.00,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.50,
	},
	{
		Family:            "claude-sonnet-4",
		InputPerMTok:      3.00,
		OutputPerMTok:     15.00,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	},
	{
		Family:            "claude-haiku-4",
		InputPerMTok:      0.80,
		OutputPerMTok:     4.00,
		CacheWritePerMTok: 1.00,
		CacheReadPerMTok:  0.08,
	},
	{
		Family:            "claude-3-opus",
		InputPerMTok:      15.00,
		OutputPerMTok:     75.00,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.50,
	},
	{
		Family:            "claude-3-5-sonnet",
		InputPerMTok:      3.00,
		OutputPerMTok:     15.00,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	},
	{
		Family:            "claude-3-sonnet",
		InputPerMTok:      3.00,
		OutputPerMTok:     15.00,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	},
	{
		Family:            "claude-3-5-haiku",
		InputPerMTok:      0.80,
		OutputPerMTok:     4.00,
		CacheWritePerMTok: 1.00,
		CacheReadPerMTok:  0.08,
	},
	{
		Family:            "claude-3-haiku",
		InputPerMTok:      0.80,
		OutputPerMTok:     4.00,
		CacheWritePerMTok: 1.00,
		CacheReadPerMTok:  0.08,
	},
}

// LookupPricing returns the best-matching pricing for a model ID using
// longest-prefix matching. Returns (zero, false) for unrecognized models.
func LookupPricing(modelID string) (ModelPricing, bool) {
	var best ModelPricing
	bestLen := -1
	for _, p := range pricingTable {
		if strings.HasPrefix(modelID, p.Family) && len(p.Family) > bestLen {
			best = p
			bestLen = len(p.Family)
		}
	}
	return best, bestLen >= 0
}

// ComputeCost returns the USD cost for the given token usage and model ID.
// Returns 0 for unrecognized model IDs.
func ComputeCost(modelID string, u TokenUsage) float64 {
	p, ok := LookupPricing(modelID)
	if !ok {
		return 0
	}
	const mtok = 1_000_000.0
	return float64(u.InputTokens)/mtok*p.InputPerMTok +
		float64(u.OutputTokens)/mtok*p.OutputPerMTok +
		float64(u.CacheCreationInputTokens)/mtok*p.CacheWritePerMTok +
		float64(u.CacheReadInputTokens)/mtok*p.CacheReadPerMTok
}

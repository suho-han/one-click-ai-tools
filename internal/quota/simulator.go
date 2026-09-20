// Package quota implements the `oct quota` feature: OpenCode Go quota
// rendering with reset countdowns plus a cache-hit-rate-based cost simulator
// over local session token stats. See
// context/opencode_quota_feature_spec.md for the feature spec.
package quota

import (
	"fmt"
	"sort"
)

// ModelPricing is per-million-token USD pricing for the cost simulator.
type ModelPricing struct {
	Name     string  `json:"name"`
	Input    float64 `json:"input"`     // $/1M cache-miss input tokens
	CacheHit float64 `json:"cache_hit"` // $/1M cache-read (hit) tokens
	Output   float64 `json:"output"`    // $/1M output tokens
}

// SimulatorModels is the default pricing table from the feature spec. Prices
// are USD per 1M tokens.
var SimulatorModels = []ModelPricing{
	{Name: "DeepSeek V4 Pro", Input: 0.66, CacheHit: 0.022, Output: 1.98},
	{Name: "Qwen3.7 Plus", Input: 0.40, CacheHit: 0.080, Output: 1.60},
	{Name: "MiMo-V2.5", Input: 0.14, CacheHit: 0.0028, Output: 0.28},
}

// highCacheHitRate is the threshold above which the spec recommends the
// model with the cheapest cache-hit pricing first (MiMo-V2.5 in the default
// table). Below it, ultra-cheap cache-hit pricing buys nothing.
const highCacheHitRate = 0.90

// cheapCacheHitPrice marks models whose appeal is predominantly ultra-cheap
// cache-hit pricing; the low-hit-rate recommendation skips them.
const cheapCacheHitPrice = 0.05

const tokensPerMillion = 1_000_000.0

// SessionTokens aggregates the raw counts the simulator consumes.
type SessionTokens struct {
	Input     int64 // cache-miss input tokens
	CacheRead int64 // cache-hit input tokens
	Output    int64
}

// CacheHitRate returns CacheRead / (Input + CacheRead), 0 when nothing was
// read.
func CacheHitRate(s SessionTokens) float64 {
	denominator := s.Input + s.CacheRead
	if denominator <= 0 {
		return 0
	}
	return float64(s.CacheRead) / float64(denominator)
}

// SimulateCost prices one model against the session token counts.
func SimulateCost(s SessionTokens, pricing ModelPricing) float64 {
	return float64(s.Input)/tokensPerMillion*pricing.Input +
		float64(s.CacheRead)/tokensPerMillion*pricing.CacheHit +
		float64(s.Output)/tokensPerMillion*pricing.Output
}

// CostEstimate is one row of the comparison table.
type CostEstimate struct {
	Model string  `json:"model"`
	Cost  float64 `json:"cost"`
	Best  bool    `json:"best"`
	Note  string  `json:"note,omitempty"`
}

// Simulate returns per-model cost estimates sorted cheapest-first and marks
// the spec's recommendation:
//
//   - cache hit rate >= 90%: cheapest model overall (with the default table
//     that is MiMo-V2.5, whose ultra-cheap cache-hit pricing dominates).
//   - lower hit rates: cheapest model that is NOT priced mainly on cache
//     hits, because the hit advantage does not materialize.
func Simulate(s SessionTokens, models []ModelPricing) []CostEstimate {
	rate := CacheHitRate(s)
	estimates := make([]CostEstimate, 0, len(models))
	for _, pricing := range models {
		estimates = append(estimates, CostEstimate{
			Model: pricing.Name,
			Cost:  SimulateCost(s, pricing),
		})
	}
	sort.SliceStable(estimates, func(i, j int) bool {
		return estimates[i].Cost < estimates[j].Cost
	})

	pricingByName := make(map[string]ModelPricing, len(models))
	for _, pricing := range models {
		pricingByName[pricing.Name] = pricing
	}

	bestIndex := -1
	for i, estimate := range estimates {
		pricing := pricingByName[estimate.Model]
		if rate >= highCacheHitRate || pricing.CacheHit >= cheapCacheHitPrice {
			bestIndex = i
			break
		}
	}
	if bestIndex >= 0 {
		savings := 0.0
		if worst := estimates[len(estimates)-1].Cost; worst > estimates[bestIndex].Cost {
			savings = (1 - estimates[bestIndex].Cost/worst) * 100
		}
		note := fmt.Sprintf("recommended (cache hit %.1f%%)", rate*100)
		if savings > 0 {
			note += fmt.Sprintf("; save %.1f%% vs %s", savings, estimates[len(estimates)-1].Model)
		}
		estimates[bestIndex].Best = true
		estimates[bestIndex].Note = note
	}
	return estimates
}

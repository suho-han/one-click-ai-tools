package quota

import (
	"math"
	"strings"
	"testing"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCacheHitRate(t *testing.T) {
	cases := []struct {
		name string
		in   SessionTokens
		want float64
	}{
		{"empty", SessionTokens{}, 0},
		{"all miss", SessionTokens{Input: 100}, 0},
		{"spec example", SessionTokens{Input: 33100000, CacheRead: 742500000}, 742.5 / (33.1 + 742.5)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CacheHitRate(tc.in); !approxEqual(got, tc.want) {
				t.Fatalf("CacheHitRate() = %f, want %f", got, tc.want)
			}
		})
	}
}

func TestSimulateCostMatchesSpecExample(t *testing.T) {
	// Spec example: 33.1M miss input, 742.5M hit, 1.8M output.
	s := SessionTokens{Input: 33_100_000, CacheRead: 742_500_000, Output: 1_800_000}

	cases := map[string]float64{
		"DeepSeek V4 Pro": 33.1*0.66 + 742.5*0.022 + 1.8*1.98,
		"Qwen3.7 Plus":    33.1*0.40 + 742.5*0.080 + 1.8*1.60,
		"MiMo-V2.5":       33.1*0.14 + 742.5*0.0028 + 1.8*0.28,
	}
	got := Simulate(s, SimulatorModels)
	if len(got) != len(SimulatorModels) {
		t.Fatalf("got %d estimates, want %d", len(got), len(SimulatorModels))
	}
	for _, estimate := range got {
		want := cases[estimate.Model]
		if !approxEqual(estimate.Cost, want) {
			t.Fatalf("%s cost = %.2f, want %.2f", estimate.Model, estimate.Cost, want)
		}
	}
}

func TestSimulate_RecommendsCheapestCacheHitModelOnHighHitRate(t *testing.T) {
	// Spec: >=90% hit rate -> MiMo-V2.5 (cheapest cache-hit pricing) first.
	s := SessionTokens{Input: 3_310_000, CacheRead: 74_250_000, Output: 180_000}

	got := Simulate(s, SimulatorModels)

	if !got[0].Best || got[0].Model != "MiMo-V2.5" {
		t.Fatalf("best = %s (best=%t), want MiMo-V2.5", got[0].Model, got[0].Best)
	}
	if !strings.Contains(got[0].Note, "save") {
		t.Fatalf("note = %q, want savings mention", got[0].Note)
	}
}

func TestSimulate_SkipsCheapCacheHitModelsOnLowHitRate(t *testing.T) {
	// Spec: low hit rates void the cache-hit advantage -> recommend among the
	// normally-priced models (Qwen3.7 Plus here), not MiMo — even though MiMo
	// is still the absolute cheapest at this mix.
	s := SessionTokens{Input: 74_250_000, CacheRead: 3_310_000, Output: 1_800_000}

	got := Simulate(s, SimulatorModels)

	var best *CostEstimate
	for i := range got {
		if got[i].Best {
			best = &got[i]
		}
	}
	if best == nil {
		t.Fatal("no estimate marked best")
	}
	if best.Model != "Qwen3.7 Plus" {
		t.Fatalf("best = %s, want Qwen3.7 Plus", best.Model)
	}
}

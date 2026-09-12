package usage

import (
	"context"
	"testing"
)

// swapStandaloneFetchers replaces the registry with a single standalone
// provider whose fetch is the given stub, restoring the original afterwards.
func swapStandaloneFetchers(t *testing.T, p Provider) {
	t.Helper()
	orig := providers
	origFetchers := providerFetchers
	providers = []Provider{p}
	providerFetchers = map[string]func(context.Context) UsageResult{p.Name: p.Fetch}
	t.Cleanup(func() {
		providers = orig
		providerFetchers = origFetchers
	})
}

func TestGetUsageStandaloneOptIn(t *testing.T) {
	fetched := false
	swapStandaloneFetchers(t, Provider{
		Name:       "standalone-test",
		Standalone: true,
		Fetch: func(context.Context) UsageResult {
			fetched = true
			return UsageResult{Provider: "standalone-test", Status: "ok"}
		},
	})

	selectOnly(t, "codex") // zai-like standalone provider NOT listed
	results, err := GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if fetched {
		t.Fatal("standalone provider fetched without explicit opt-in")
	}
	for _, r := range results {
		if r.Provider == "standalone-test" {
			t.Fatalf("standalone provider leaked into results: %+v", results)
		}
	}
}

func TestGetUsageStandaloneFetchedWhenListed(t *testing.T) {
	swapStandaloneFetchers(t, Provider{
		Name:       "standalone-test",
		Standalone: true,
		Fetch: func(context.Context) UsageResult {
			return UsageResult{Provider: "standalone-test", Status: "ok", Used: "1"}
		},
	})

	selectOnly(t, "standalone-test")
	results, err := GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	var found bool
	for _, r := range results {
		if r.Provider == "standalone-test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("listed standalone provider missing from results: %+v", results)
	}
}

func TestGetUsageStandaloneFetchedByAlias(t *testing.T) {
	swapStandaloneFetchers(t, Provider{
		Name:       "standalone-test",
		Aliases:    []string{"testalias"},
		Standalone: true,
		Fetch: func(context.Context) UsageResult {
			return UsageResult{Provider: "standalone-test", Status: "ok"}
		},
	})

	selectOnly(t, "testalias")
	results, err := GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	var found bool
	for _, r := range results {
		if r.Provider == "standalone-test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("standalone provider not fetched via alias: %+v", results)
	}
}

func TestGetUsageHonorsRequestedOrderForStandalone(t *testing.T) {
	// Nested swaps: standalone swap saves the real registry, fetcher swap then
	// saves the standalone-only fetcher map; cleanups unwind in reverse.
	swapStandaloneFetchers(t, Provider{
		Name:       "zai",
		Standalone: true,
		Fetch: func(context.Context) UsageResult {
			return UsageResult{Provider: "zai", Status: "ok"}
		},
	})
	swapFetchers(t, map[string]func(context.Context) UsageResult{
		"claude": func(context.Context) UsageResult { return UsageResult{Provider: "claude", Status: "ok"} },
		"codex":  func(context.Context) UsageResult { return UsageResult{Provider: "codex", Status: "ok"} },
	})

	selectOnly(t, "zai", "codex", "claude")
	results, err := GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	want := []string{"zai", "codex", "claude"}
	if len(results) != len(want) {
		t.Fatalf("results = %+v, want %v", results, want)
	}
	for i, name := range want {
		if results[i].Provider != name {
			t.Fatalf("results[%d].Provider = %q, want %q (full order: %v)", i, results[i].Provider, name, results)
		}
	}
}

func TestProviderRequested(t *testing.T) {
	requested := requestedProviderNames()
	if len(requested) != 0 {
		t.Fatalf("requestedProviderNames() with no config = %v, want empty", requested)
	}

	p := Provider{Name: "zai", Aliases: []string{"glm"}}
	if providerRequested(p, map[string]bool{"codex": true}) {
		t.Fatal("providerRequested true for unrelated name")
	}
	if !providerRequested(p, map[string]bool{"zai": true}) {
		t.Fatal("providerRequested false for exact name")
	}
	if !providerRequested(p, map[string]bool{"glm": true}) {
		t.Fatal("providerRequested false for alias")
	}
}

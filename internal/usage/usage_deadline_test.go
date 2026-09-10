package usage

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// selectOnly pins SelectedTools to the given binary names so a test can
// substitute fetchers by key.
func selectOnly(t *testing.T, names ...string) {
	t.Helper()
	viper.Set("agent_order", names)
	viper.Set("enabled_tools", names)
	t.Cleanup(func() {
		viper.Set("agent_order", nil)
		viper.Set("enabled_tools", nil)
	})
}

func swapFetchers(t *testing.T, fetchers map[string]func(context.Context) UsageResult) {
	t.Helper()
	orig := providerFetchers
	origTimeout := usageFetchTimeout
	providerFetchers = fetchers
	t.Cleanup(func() {
		providerFetchers = orig
		usageFetchTimeout = origTimeout
	})
}

// TestGetUsageDeadlineReturnsTimeoutEntries pins the return rule: when the
// overall ceiling fires, providers still running get an error UsageResult,
// finished providers keep their results, and GetUsage returns promptly.
func TestGetUsageDeadlineReturnsTimeoutEntries(t *testing.T) {
	selectOnly(t, "codex", "claude")

	fastResult := UsageResult{Provider: "codex", Status: "ok", Used: "12.5", Unit: "percent"}
	swapFetchers(t, map[string]func(context.Context) UsageResult{
		"codex": func(context.Context) UsageResult { return fastResult },
		"claude": func(ctx context.Context) UsageResult {
			<-ctx.Done() // simulate a fetcher that would hang until killed
			return UsageResult{Provider: "claude", Status: "ok"}
		},
	})
	usageFetchTimeout = 100 * time.Millisecond

	start := time.Now()
	results, err := GetUsage(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("GetUsage() took %v, want bounded by the 100ms ceiling", elapsed)
	}

	byProvider := map[string]UsageResult{}
	for _, r := range results {
		byProvider[r.Provider] = r
	}

	got, ok := byProvider["codex"]
	if !ok || got.Used != "12.5" || got.Status != "ok" {
		t.Fatalf("fast provider result not preserved: %+v", byProvider["codex"])
	}

	slow, ok := byProvider["claude"]
	if !ok {
		t.Fatalf("slow provider missing timeout entry: %+v", results)
	}
	if slow.Status != "error" || !strings.Contains(slow.Message, "timed out") {
		t.Fatalf("slow provider entry = %+v, want error status with timeout message", slow)
	}
}

// TestGetUsageMixedFastAndSlow is the user-visible half of the contract: the
// JSON payload still carries every provider after a deadline hit.
func TestGetUsageMixedFastAndSlow(t *testing.T) {
	selectOnly(t, "codex", "claude", "copilot")
	swapFetchers(t, map[string]func(context.Context) UsageResult{
		"codex": func(context.Context) UsageResult { return UsageResult{Provider: "codex", Status: "ok"} },
		"claude": func(ctx context.Context) UsageResult {
			<-ctx.Done()
			return UsageResult{Provider: "claude", Status: "ok"}
		},
		"copilot": func(context.Context) UsageResult { return UsageResult{Provider: "copilot", Status: "warn"} },
	})
	usageFetchTimeout = 100 * time.Millisecond

	results, err := GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("GetUsage() returned %d results, want 3 (all providers present)", len(results))
	}
}

// TestFetchClaudeUsageHungKeychainKilledByContext proves a hung keychain
// helper cannot stall the fetch: execCommand is ctx-aware and the lookup is
// abandoned (falling through to the next token source) when the context dies.
func TestFetchClaudeUsageHungKeychainKilledByContext(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "security" {
			// sleep only ends when ctx kills it at the deadline
			return exec.CommandContext(ctx, "sleep", "30")
		}
		return exec.Command(name, args...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	done := make(chan UsageResult, 1)
	go func() { done <- FetchClaudeUsage(ctx) }()

	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("FetchClaudeUsage took %v with a hung keychain, want bounded by ctx", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("FetchClaudeUsage still blocked after 10s: hung keychain not killed by ctx")
	}
}

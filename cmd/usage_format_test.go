package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

// runUsageWithFormat drives usageCmd.RunE with an isolated flag state (the
// flag set is a package-level singleton, so every explicit value is set
// fresh) and returns trimmed stdout.
func runUsageWithFormat(t *testing.T, setFlags func(*testing.T)) string {
	t.Helper()
	setFlags(t)
	out := captureCommandStdout(t, func() {
		if err := usageCmd.RunE(usageCmd, nil); err != nil {
			t.Fatalf("usage command errored: %v", err)
		}
	})
	return out
}

func usageFlagSetter(t *testing.T, args ...string) func(*testing.T) {
	t.Helper()
	return func(t *testing.T) {
		for _, name := range []string{"json", "compact", "from-snapshot"} {
			if f := usageCmd.Flags().Lookup(name); f != nil {
				if err := f.Value.Set("false"); err != nil {
					t.Fatalf("reset --%s: %v", name, err)
				}
				f.Changed = false
			}
		}
		if f := usageCmd.Flags().Lookup("format"); f != nil {
			if err := f.Value.Set(""); err != nil {
				t.Fatalf("reset --format: %v", err)
			}
			f.Changed = false
		}
		cmd := usageCmd
		cmd.SetArgs(args)
		if err := cmd.Flags().Parse(args); err != nil {
			t.Fatalf("parse flags %v: %v", args, err)
		}
	}
}

func TestUsageCommandFormatWaybar(t *testing.T) {
	orig := usageFetcher
	usageFetcher = func(ctx context.Context) ([]usage.UsageResult, error) {
		return []usage.UsageResult{
			{Provider: "codex", Status: "ok", Unit: "percent", Used: "80.0", Buckets: map[string]string{"7d": "55.0"}},
			{Provider: "claude", Status: "ok", Unit: "percent", Used: "12.0", Buckets: map[string]string{"5h": "12.0"}},
		}, nil
	}
	defer func() { usageFetcher = orig }()

	out := runUsageWithFormat(t, usageFlagSetter(t, "--format", "waybar"))
	var payload struct {
		Text  string `json:"text"`
		Class string `json:"class"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("waybar output not JSON: %v\nraw: %s", err, out)
	}
	if payload.Class != "ok" || !strings.Contains(payload.Text, "X-") {
		t.Fatalf("unexpected waybar payload: %+v", payload)
	}
}

func TestUsageCommandFormatFromSnapshot(t *testing.T) {
	orig := usageFetcher
	usageFetcher = func(ctx context.Context) ([]usage.UsageResult, error) {
		return nil, errors.New("live fetch must not run when --from-snapshot is set")
	}
	defer func() { usageFetcher = orig }()

	snapshotPath := t.TempDir() + "/usage-latest.json"
	if err := usage.SaveSnapshot(snapshotPath, []usage.UsageResult{
		{Provider: "codex", Status: "ok", Unit: "percent", Used: "10.0", Buckets: map[string]string{"7d": "10.0"}},
	}, time.Now()); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	out := runUsageWithFormat(t, func(t *testing.T) {
		usageFlagSetter(t, "--format", "compact", "--from-snapshot", "--snapshot-path", snapshotPath)(t)
	})
	if out != "X-90%" {
		t.Fatalf("snapshot compact output = %q, want X-90%% (remaining of 7d 10)", out)
	}
}

func TestUsageCommandFromSnapshotRequiresFormat(t *testing.T) {
	err := func() error {
		usageFlagSetter(t, "--from-snapshot")(t)
		return usageCmd.RunE(usageCmd, nil)
	}()
	if err == nil || !strings.Contains(err.Error(), "--from-snapshot requires") {
		t.Fatalf("err = %v, want from-snapshot-requires-format", err)
	}
}

func TestUsageCommandRejectsInvalidFormat(t *testing.T) {
	err := func() error {
		usageFlagSetter(t, "--format", "fancy")(t)
		return usageCmd.RunE(usageCmd, nil)
	}()
	if err == nil || !strings.Contains(err.Error(), "invalid --format") {
		t.Fatalf("err = %v, want invalid --format error", err)
	}
}

func TestUsageCommandFormatSwiftBarHonorsDisplayMode(t *testing.T) {
	orig := usageFetcher
	usageFetcher = func(ctx context.Context) ([]usage.UsageResult, error) {
		return []usage.UsageResult{
			{Provider: "codex", Status: "ok", Unit: "percent", Used: "55.0", Buckets: map[string]string{"7d": "55.0"}},
		}, nil
	}
	defer func() { usageFetcher = orig }()

	oldMode := viper.GetString("usage_display_mode")
	t.Cleanup(func() { viper.Set("usage_display_mode", oldMode) })
	viper.Set("usage_display_mode", "used")

	out := runUsageWithFormat(t, usageFlagSetter(t, "--format", "swiftbar"))
	if !strings.HasPrefix(out, "X-55% | color=") {
		t.Fatalf("swiftbar output = %q, want used-mode title with color param", out)
	}
	if !strings.Contains(out, "---") {
		t.Fatalf("swiftbar output = %q, want dropdown separator", out)
	}
}

package cmd

import (
	"testing"

	"github.com/suho-han/one-click-tools/internal/usage"
)

func TestSortMonitorResults(t *testing.T) {
	in := []usage.UsageResult{
		{Provider: "b", Unit: "percent", Used: "88", Buckets: map[string]string{"5h": "80"}},
		{Provider: "a", Unit: "percent", Used: "92", Buckets: map[string]string{"5h": "95"}},
	}
	out := sortMonitorResults(in, "used", true)
	if out[0].Provider != "a" {
		t.Fatalf("expected provider a first, got %s", out[0].Provider)
	}
}

func TestUsageSeverity(t *testing.T) {
	if got := usageSeverity(usage.UsageResult{Unit: "percent", Used: "70"}); got != "OK" {
		t.Fatalf("expected OK, got %s", got)
	}
	if got := usageSeverity(usage.UsageResult{Unit: "percent", Used: "88"}); got != "WARN" {
		t.Fatalf("expected WARN, got %s", got)
	}
	if got := usageSeverity(usage.UsageResult{Unit: "percent", Used: "99"}); got != "CRIT" {
		t.Fatalf("expected CRIT, got %s", got)
	}
}

package update

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestProbeMemoDeduplicatesDetectionProbes(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	orig := commandOutput
	defer func() { commandOutput = orig }()
	commandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		mu.Lock()
		calls[name+" "+strings.Join(args, " ")]++
		mu.Unlock()
		return nil, errExecutableNotFound
	}

	ctx := withProbeMemo(context.Background())
	tool := Tool{Name: "foo", BinaryName: "foo", Package: "foo", BinaryAliases: []string{"foo-bar"}}
	_, _ = explainResolvedManager(ctx, tool)
	_ = DetectManager(ctx, tool)
	_ = DetectManager(ctx, tool)

	for key, count := range calls {
		if count > 1 {
			t.Fatalf("probe %q ran %d times under one memo; expected 1", key, count)
		}
	}
}

func TestProbeMemoWithoutContextStillProbes(t *testing.T) {
	var mu sync.Mutex
	var count int
	orig := commandOutput
	defer func() { commandOutput = orig }()
	commandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		mu.Lock()
		count++
		mu.Unlock()
		return nil, errExecutableNotFound
	}

	tool := Tool{Name: "foo", BinaryName: "foo", Package: "foo"}
	_ = DetectManager(context.Background(), tool)
	_ = DetectManager(context.Background(), tool)

	mu.Lock()
	got := count
	mu.Unlock()
	if got < 2 {
		t.Fatalf("expected repeated probes without a memo context, got %d calls", got)
	}
}

func TestWithProbeMemoIsIdempotent(t *testing.T) {
	ctx := withProbeMemo(context.Background())
	if withProbeMemo(ctx) != ctx {
		t.Fatal("expected withProbeMemo to return an existing memo context unchanged")
	}
}

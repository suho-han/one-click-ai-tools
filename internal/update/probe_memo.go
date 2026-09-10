package update

import (
	"context"
	"strings"
	"sync"
)

// probeMemoKey carries the per-detection-phase probe cache in a context.
type probeMemoKey struct{}

// probeMemo memoizes manager-detection subprocess probes (manager prefixes,
// installed-package lookups) for one detection phase. Detection runs the same
// probe set from multiple entry points (ExplainPlans, anyBrewManaged ->
// ResolveManagerForInstall), which otherwise shells out dozens of times per
// run. The cache never spans an install: results change once a manager
// upgrades a tool, so the install loop runs on a memo-free context.
type probeMemo struct {
	mu      sync.Mutex
	entries map[string]memoEntry
}

type memoEntry struct {
	out []byte
	err error
}

// withProbeMemo returns a context whose detection probes share one cache.
func withProbeMemo(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if probeMemoFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, probeMemoKey{}, &probeMemo{entries: make(map[string]memoEntry)})
}

func probeMemoFromContext(ctx context.Context) *probeMemo {
	if ctx == nil {
		return nil
	}
	memo, _ := ctx.Value(probeMemoKey{}).(*probeMemo)
	return memo
}

func memoKey(name string, args []string) string {
	return name + "\x00" + strings.Join(args, "\x00")
}

// memoizedCommandOutput consults the context's probe cache before shelling
// out, and records the result (including probe errors, which are the common
// case for managers that are not installed).
func memoizedCommandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	memo := probeMemoFromContext(ctx)
	if memo == nil {
		return commandOutput(ctx, name, args...)
	}
	key := memoKey(name, args)
	memo.mu.Lock()
	entry, ok := memo.entries[key]
	memo.mu.Unlock()
	if ok {
		return entry.out, entry.err
	}
	out, err := commandOutput(ctx, name, args...)
	memo.mu.Lock()
	memo.entries[key] = memoEntry{out: out, err: err}
	memo.mu.Unlock()
	return out, err
}

package quota

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCollectSessionTokens_AggregatesCodexAndClaudeLogs(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	home := t.TempDir()

	// Codex: totals are cumulative per session, so the LAST token_count
	// event wins (100+200+400 in-file drift must NOT be summed).
	writeFile(t, filepath.Join(home, ".codex", "sessions", "a.jsonl"),
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":10}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":200,"cache_read_input_tokens":400,"output_tokens":20}}}}
`)
	// Codex real-data shape: input_tokens INCLUDES cached input
	// (24541 = 23133 miss + 1408 cached; total_tokens == input + output).
	writeFile(t, filepath.Join(home, ".codex", "sessions", "b.jsonl"),
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":24541,"cached_input_tokens":1408,"output_tokens":5,"total_tokens":24546}}}}
`)
	// Codex file with no token events: counts as a file, not a session.
	writeFile(t, filepath.Join(home, ".codex", "sessions", "empty.jsonl"),
		`{"type":"event_msg","payload":{"type":"other"}}
`)
	// Claude: per-message usage is summed; cache_creation folds into Input.
	writeFile(t, filepath.Join(home, ".claude", "projects", "p", "s.jsonl"),
		`{"message":{"usage":{"input_tokens":1000,"cache_read_input_tokens":9000,"cache_creation_input_tokens":500,"output_tokens":100}}}
{"message":{"usage":{"input_tokens":2000,"cache_read_input_tokens":3000,"output_tokens":200}}}
`)
	// An old claude log outside the window must be skipped.
	old := filepath.Join(home, ".claude", "projects", "p", "old.jsonl")
	writeFile(t, old, `{"message":{"usage":{"input_tokens":999999,"output_tokens":999999}}}`)
	past := time.Now().AddDate(0, 0, -40)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	stats, err := CollectSessionTokens(home, 30, time.Now())
	if err != nil {
		t.Fatalf("CollectSessionTokens() error: %v", err)
	}

	if stats.Files != 4 {
		t.Fatalf("files = %d, want 4 (empty codex log counts, old claude log skipped)", stats.Files)
	}
	if stats.Sessions != 3 {
		t.Fatalf("sessions = %d, want 3", stats.Sessions)
	}
	// codex: (0 miss, 400 hit, 20 out) + (23133 miss, 1408 hit, 5 out);
	// claude: (3500 miss, 12000 hit, 300 out).
	if stats.Tokens.Input != 26633 || stats.Tokens.CacheRead != 13808 || stats.Tokens.Output != 325 {
		t.Fatalf("tokens = %+v, want input 26633 cacheRead 13808 output 325", stats.Tokens)
	}
}

func TestCollectSessionTokens_MissingDirectoriesAreZero(t *testing.T) {
	stats, err := CollectSessionTokens(t.TempDir(), 7, time.Now())
	if err != nil {
		t.Fatalf("CollectSessionTokens() error: %v", err)
	}
	if stats.Sessions != 0 || stats.Files != 0 {
		t.Fatalf("stats = %+v, want zero", stats)
	}
}

func TestCollectSessionTokens_LargeJsonlLinesDoNotAbortScan(t *testing.T) {
	home := t.TempDir()
	huge := strings.Repeat("x", 2*1024*1024)
	writeFile(t, filepath.Join(home, ".claude", "projects", "p", "s.jsonl"),
		`{"message":{"usage":{"input_tokens":5}}}`+"\n"+`{"padding":"`+huge+`"}`+"\n"+
			`{"message":{"usage":{"input_tokens":7,"output_tokens":3}}}`+"\n")

	stats, err := CollectSessionTokens(home, 30, time.Now())
	if err != nil {
		t.Fatalf("CollectSessionTokens() error: %v", err)
	}
	if stats.Tokens.Input != 12 || stats.Tokens.Output != 3 {
		t.Fatalf("tokens = %+v, want input 12 output 3", stats.Tokens)
	}
}

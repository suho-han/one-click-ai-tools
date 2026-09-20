package quota

import (
	"bufio"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// scanLines feeds every line of the file to handle, including lines larger
// than any buffer cap — a session log with one giant event must still
// contribute its other events.
func scanLines(reader io.Reader, handle func(line []byte) error) error {
	buffered := bufio.NewReader(reader)
	for {
		line, err := buffered.ReadString('\n')
		if line != "" {
			if handleErr := handle([]byte(line)); handleErr != nil {
				return handleErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// SessionStats is the aggregate over local session logs within the window.
type SessionStats struct {
	Days     int           `json:"days"`
	Since    time.Time     `json:"since"`
	Files    int           `json:"files"`    // log files scanned in the window
	Sessions int           `json:"sessions"` // files that carried token data
	Tokens   SessionTokens `json:"tokens"`
}

// codexTokenCountLine matches Codex session-log token_count events. Codex
// reports cumulative totals per session, so the LAST event in a file carries
// that session's totals.
type codexTokenCountLine struct {
	Type    string `json:"type"`
	Payload struct {
		Type string `json:"type"`
		Info struct {
			TotalTokenUsage struct {
				InputTokens          int64 `json:"input_tokens"`
				CachedInputTokens    int64 `json:"cached_input_tokens"`
				CacheReadInputTokens int64 `json:"cache_read_input_tokens"`
				OutputTokens         int64 `json:"output_tokens"`
			} `json:"total_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

// claudeUsageLine matches Claude Code project-log assistant entries, which
// carry per-message (non-cumulative) usage.
type claudeUsageLine struct {
	Message struct {
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// codexSessionsDir mirrors codexHomePath in internal/usage/codex.go so
// quota stats sees the same sessions the codex usage provider does.
func codexSessionsDir(home string) string {
	if custom := strings.TrimSpace(os.Getenv("CODEX_HOME")); custom != "" {
		return filepath.Join(custom, "sessions")
	}
	return filepath.Join(home, ".codex", "sessions")
}

// CollectSessionTokens aggregates cache-relevant token counts from local
// coding-tool session logs modified within the last `days` days. Sources:
//
//   - $CODEX_HOME/sessions (default ~/.codex/sessions)/**/*.jsonl —
//     token_count events with cumulative totals; the last event per file
//     is that session's total.
//   - ~/.claude/projects/**/*.jsonl — per-message usage; all events are
//     summed. Cache-creation tokens count as input (cache misses).
//
// Missing source directories are not errors — they simply contribute zero.
func CollectSessionTokens(home string, days int, now time.Time) (SessionStats, error) {
	since := now.AddDate(0, 0, -days)
	stats := SessionStats{Days: days, Since: since}

	for _, source := range []struct {
		dir     string
		collect func(path string) (SessionTokens, bool, error)
	}{
		{dir: codexSessionsDir(home), collect: collectCodexFileTokens},
		{dir: filepath.Join(home, ".claude", "projects"), collect: collectClaudeFileTokens},
	} {
		err := filepath.WalkDir(source.dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// A missing source dir is fine; anything else aborts.
				if os.IsNotExist(err) {
					return fs.SkipDir
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.ModTime().Before(since) {
				return nil
			}
			stats.Files++
			tokens, hasData, err := source.collect(path)
			if err != nil {
				return err
			}
			if hasData {
				stats.Sessions++
				stats.Tokens.Input += tokens.Input
				stats.Tokens.CacheRead += tokens.CacheRead
				stats.Tokens.Output += tokens.Output
			}
			return nil
		})
		if err != nil {
			return stats, err
		}
	}
	return stats, nil
}

// collectCodexFileTokens returns the totals of the LAST token_count event in
// the file (Codex totals are cumulative per session).
func collectCodexFileTokens(path string) (SessionTokens, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return SessionTokens{}, false, err
	}
	defer file.Close()

	var last SessionTokens
	found := false
	err = scanLines(file, func(raw []byte) error {
		var line codexTokenCountLine
		if err := json.Unmarshal(raw, &line); err != nil {
			return nil
		}
		if line.Type != "event_msg" || line.Payload.Type != "token_count" {
			return nil
		}
		usage := line.Payload.Info.TotalTokenUsage
		// OpenAI-style accounting: input_tokens already includes cached
		// input (total_tokens == input_tokens + output_tokens), so the
		// cache-miss figure is what remains after subtracting it.
		cacheRead := usage.CacheReadInputTokens
		if cacheRead == 0 {
			cacheRead = usage.CachedInputTokens
		}
		missInput := usage.InputTokens - cacheRead
		if missInput < 0 {
			missInput = 0
		}
		last = SessionTokens{Input: missInput, CacheRead: cacheRead, Output: usage.OutputTokens}
		found = true
		return nil
	})
	return last, found, err
}

// collectClaudeFileTokens sums the per-message usage events (cache-creation
// tokens are writes, i.e. cache misses, and fold into Input).
func collectClaudeFileTokens(path string) (SessionTokens, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return SessionTokens{}, false, err
	}
	defer file.Close()

	var total SessionTokens
	found := false
	err = scanLines(file, func(raw []byte) error {
		var line claudeUsageLine
		if err := json.Unmarshal(raw, &line); err != nil {
			return nil
		}
		usage := line.Message.Usage
		if usage.InputTokens == 0 && usage.CacheReadInputTokens == 0 && usage.CacheCreationInputTokens == 0 && usage.OutputTokens == 0 {
			return nil
		}
		total.Input += usage.InputTokens + usage.CacheCreationInputTokens
		total.CacheRead += usage.CacheReadInputTokens
		total.Output += usage.OutputTokens
		found = true
		return nil
	})
	return total, found, err
}

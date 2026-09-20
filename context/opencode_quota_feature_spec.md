# OpenCode Go Quota Monitoring & Cost Simulator — Feature Spec

> **Status: implemented (2026-09-18, dev).** `oct quota show` reuses the
> usage provider's OpenCode Go client (`internal/usage/opencode.go`) instead
> of a duplicate `internal/quota/client.go`; session parsing and the cost
> simulator live in `internal/quota` (parser.go / simulator.go). Pricing
> overrides are in-code constants (`SimulatorModels`); the API endpoint is
> overridable via `oct quota show --endpoint` / `OCT_OPENCODE_USAGE_ENDPOINT`.

This document is the development spec and planning note for adding real-time
OpenCode Go quota lookup and a cache-hit-rate-based cost simulator to
`one-click-ai-tools` as a one-click CLI command.

---

## 1. Overview & Background

* **Background**: Coding-agent sessions reach 70K–120K context tokens, burning
  quota extremely fast. Users need terminal visibility into OpenCode Go quota
  limits (rolling 5-hour $12, weekly $30, monthly $60) so they can react
  immediately.
* **Goal**: Add an `oct quota` command that bundles current quota consumption,
  reset countdowns, and a cost simulation for alternative models (e.g.
  MiMo-V2.5) based on local usage history.

---

## 2. Requirements

### 2.1 Auth & settings detection
* **Environment variable detection**: auto-detect `OPENCODE_API_KEY` in the
  terminal; if absent, parse it from `~/.zshrc` or `~/.env` first.
* **Config file sync**: diagnose whether `~/.config/opencode/opencode.json`
  contains the `provider.opencode-go` settings and confirm account linkage.

### 2.2 Real-time quota monitoring (TUI)
* **Quota lookup**: query the OpenCode Go API endpoint
  (`https://opencode.ai/zen/go/v1` or the `opencode-quota` integration API)
  for remaining allowance.
* **Progress bar rendering**: render remaining percentages for Rolling 5-Hour
  ($12), Weekly ($30), and Monthly ($60) as terminal progress bars
  (`████░░░░ 50% left`).
* **Reset countdown**: show time remaining until the rolling reset
  (e.g. ~3h 42m) visually.

### 2.3 Cache-hit-rate-based cost simulator
* **Local stats parsing**: parse local session data (Input, Output, Cache Read
  token counts) and compute the session's cache hit rate
  (`Cache Read / (Input + Cache Read)`).
* **Scenario comparison**:
  * **DeepSeek V4 Pro**: `Input $0.66/1M`, `Hit $0.022/1M`, `Output $1.98/1M`
  * **Qwen3.7 Plus**: `Input $0.40/1M`, `Hit $0.080/1M`, `Output $1.60/1M`
  * **MiMo-V2.5**: `Input $0.14/1M`, `Hit $0.0028/1M`, `Output $0.28/1M`
  Apply these unit prices and compare the estimated dollar cost of each.
* **Automatic best-model recommendation**:
  * For agent sessions with an extremely high cache hit rate (≥90%), recommend
    **MiMo-V2.5** first — it has the cheapest cache-hit pricing.
  * For low cache hit rates or one-off usage, recommend **Qwen3.7 Plus** or
    **MiniMax M3**.

---

## 3. Go architecture & implementation direction

### 3.1 CLI command registration
* File path: `cmd/quota.go`
* Enter the feature via subcommands under `oct quota`:
  * `oct quota show`: render real-time quota bars.
  * `oct quota stats`: print local session summary and the cost simulator.

### 3.2 Internal package layout
* **`internal/quota/client.go`**: API auth and HTTP transport.
* **`internal/quota/parser.go`**: local session token-log analysis and cache
  hit-rate computation.
* **`internal/quota/simulator.go`**: per-model pricing engine that computes
  scenario costs and produces recommendations.

---

## 4. Expected UI/UX output example

```bash
$ oct quota show
[OpenCode Go Quota Status]
- Rolling 5-Hour: ██████████████████████░░░░░░░░░░░░░░░░░ 57% left (Reset in 3h 12m)
- Weekly Cap:     ██████████████████████████████████░░░░ 87% left (Reset in 2d)
- Monthly Cap:    ████████████████████████████████████░░ 94% left (Reset in 14d)

$ oct quota stats
[Local Session Stats - Last 17 Days]
- Cache Miss Input: 33.1M tokens
- Cache Hit Input:  742.5M tokens
- Output Generated: 1.8M tokens
- Actual Cache Hit Rate: 95.7%

[Estimated Cost Comparison]
- DeepSeek V4 Pro: $41.75
- Qwen3.7 Plus:    $75.52 (High cache-hit penalty)
- MiMo-V2.5:       $7.22  (Highly Recommended! Save 82.7%)
```

---
*Note: the feature must allow overriding `baseURL` and `coefficients` via CLI
flags or config so it can adapt to changes in the Z.ai / OpenCode Go API
spec.*

# `oct usage --json` — Output Contract

This document is the stable contract for the JSON emitted by `oct usage --json`.
It exists because scriptability is a stated differentiator: tools that parse
this output (CI checks, status bars, cron jobs) should be able to upgrade `oct`
without breakage.

**Contract version:** as of v0.1.6-beta.3 (2026-09-18).

## Stability promise

Guaranteed:

- Field names, JSON types, and the presence rules below (required vs optional).
- The three-value `status` vocabulary: `ok`, `warn`, `error`.
- Additive evolution only: new providers, new optional fields, and new
  `buckets` / `bucket_resets` keys may appear at any time.

Not guaranteed (treat as best-effort input):

- The set of `buckets` keys for any given provider (a provider may add a
  window, or report fewer windows on a given run).
- Which providers appear in `results` (driven by the user's `enabled_tools` /
  `agent_order` config).

Breaking changes (removing or renaming a field, changing a type) will be
documented in the release notes of the version that makes them.

## Top-level shape

```json
{
  "summary": { "total": 5, "ok": 5, "warn": 0, "error": 0 },
  "results": [ { … }, … ]
}
```

- `summary.total` equals `len(results)`.
- `summary.ok` / `warn` / `error` partition `total`.
- `results` is always present; it is empty (not `null`) when no enabled
  provider produced a result.

### `summary` counting rule

A result counts as `ok` only when its `status` is `ok` **and** it carries real
data: an empty or `n/a` `used` value, or a message signalling missing/partial
data, downgrades the summary bucket to `warn` even when `status` is `ok`.
Consumers wanting the fine-grained view should read `results[].status`
directly and treat message heuristics as internal.

## `results[]` fields

| Field | Type | Presence | Meaning |
| --- | --- | --- | --- |
| `provider` | string | always | Canonical provider id (e.g. `claude-code`, `codex`, `gemini`). Stable across versions. |
| `plan` | string | optional | Subscription plan name as reported by the provider (e.g. `pro`, `prolite`). |
| `plan_source` | string | optional | Where the plan was detected from (diagnostic aid, free-form text). |
| `status` | string | always | `ok`, `warn`, or `error`. `warn` = fetched with caveats (partial data, fallback path, credentials about to fail); `error` = no usable data. |
| `used` | string | always | Usage value as a **decimal string** (not a number), e.g. `"42.5"`. Empty string or `"n/a"` when no data. String-on-purpose: it preserves provider formatting and distinguishes `0` from unknown. |
| `unit` | string | always | `percent`, `requests`, `sessions`, `msgs`, or `USD`. New units may be added. |
| `buckets` | map[string]string | optional | Per-window values, same string convention as `used`. Window keys seen today: `5h`, `7d`, `1m`; each provider emits its own subset. |
| `bucket_resets` | map[string]string | optional | Per-window reset timestamps. **Format is currently provider-dependent**: RFC 3339 (e.g. `2026-09-26T08:00:00+09:00`) for some providers, unix-epoch-seconds-as-string (e.g. `1790412359`) for others. Do not assume one format; normalize by checking the first character for `[0-9]`. |
| `source_detail` | string | optional | Diagnostic detail about the credential/refresh state used for the fetch. Free-form; not a stable vocabulary. |
| `message` | string | optional | Short human-readable note, **truncated to 48 characters** in JSON output. Not a stable vocabulary — parse at your own risk. |

Fields marked optional are omitted (Go `omitempty`) when empty; never rely on
them being present with an empty value.

## Example (trimmed)

```json
{
  "summary": { "total": 2, "ok": 2, "warn": 0, "error": 0 },
  "results": [
    {
      "provider": "codex",
      "plan": "prolite",
      "plan_source": "codex backend wham/usage",
      "status": "ok",
      "used": "0.0",
      "unit": "percent",
      "buckets": { "7d": "0.0" },
      "bucket_resets": { "7d": "1790412359" },
      "message": "Usage fetched from Codex backend API (weekly …)"
    },
    {
      "provider": "claude-code",
      "plan": "pro",
      "plan_source": "keychain subscriptionType",
      "status": "ok",
      "used": "0.0",
      "unit": "percent",
      "buckets": { "5h": "0.0", "7d": "0.0" },
      "bucket_resets": { "7d": "2026-09-26T08:00:00+09:00" },
      "source_detail": "token_expired=true;can_refresh=true",
      "message": "Usage parsed from claude --print /usage (oaut…"
    }
  ]
}
```

## Recommended consumer practice

- Ignore unknown fields; they may appear without a release note.
- Treat missing optional fields as absent, not as empty strings.
- Parse `used` / bucket values with a lenient decimal parser; compare only
  values with the same `unit`.
- When rendering reset countdowns, accept both RFC 3339 and epoch-seconds
  strings (see `bucket_resets` above).

## Related docs

- [usage.md](usage.md): command reference for `oct usage` (flags, env vars, mock endpoints for testing)
- [roadmap_2026-09-12.md](roadmap_2026-09-12.md): this doc closes the P1 item "Document `usage --json` schema as a stable contract"
- source of truth: `PrintJSON` in `internal/usage/usage.go` — when changing the payload shape, update that function and this document together

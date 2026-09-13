# Usage-Provider Addition Checklist

Every item below traces to a real defect found while adding the 2026-09
providers (kimi/qwen/minimax + standalone zai/deepseek/openrouter/grok) —
twenty findings across four codex review passes on PR #3, 2026-09-12. Run
through the whole list when adding or changing a usage provider; most items
are only checkable by hand because the failure is a silent mismatch between
two surfaces.

## 1. Registry & fetcher wiring

- [ ] The registry `Provider.Name` equals the update tool's `BinaryName`, or
  every tool binary name is listed in `Aliases`. `GetUsage` resolves
  installable providers by `update.Tool.BinaryName`, so a provider whose key
  does not match never fetches (MiniMax shipped with only a `minimax` key
  while its tool is `mmx`).
- [ ] Names the user can type in `agent_order`/`enabled_tools`/`config set
  tools` resolve end to end: add a `NormalizeToolName` case when the config
  spelling differs from the binary name.
- [ ] `Standalone: true` only for plan/account services with no installable
  CLI; verify a default config never shows a "not configured" row for it.

## 2. Unit semantics (the #1 repeat failure)

`oct usage` has three consumers that all branch on `UsageResult.Unit` —
remaining-mode display inverts only `percent`, compact output prints `?` for
non-percent, and the alert evaluator ignores non-percent. Every provider
whose data can express a ratio MUST report `Unit: "percent"`:

- [ ] Each window/bucket is a percentage of its own limit (kimi, minimax,
  qwen all shipped raw counts and broke remaining mode/alerts).
- [ ] `Limit` stays on the same scale as `Used` — for percent units set
  `Limit: "100"`, never the raw request/credit total.
- [ ] The raw count survives somewhere user-visible (message/plan) when the
  conversion loses information.
- [ ] Qwen-style local-count providers count against their configured cap and
  normalize the same way.

## 3. Credentials, endpoints, resilience

- [ ] Default credential path matches the CLI's real layout (kimi read
  `~/credentials/...` instead of `~/.kimi-code/credentials/...` when the home
  env var was unset) — verify with the env var unset, not only set.
- [ ] Region/env-driven base URLs are actually used by the fetch path; a
  helper that is never called is the failure mode (minimax `MINIMAX_REGION`).
- [ ] `POST` bodies work with `netclient.DoWithRetry` (bytes.Reader bodies
  reset via `GetBody`).
- [ ] Optional enrichment calls (plan tier, settings) get their own short
  timeout so a slow optional endpoint cannot hold the row until the shared
  usage deadline replaces it (grok settings).
- [ ] Mixed JSON shapes tolerated where the API is known to vary (numbers as
  strings, `data` envelopes, quoted timestamps — qwen timestamps failed
  `time.Parse` because the JSON quotes were never decoded).
- [ ] `OCT_*_USAGE_ENDPOINT` override documented in `usage.md` and working
  for mock testing.

## 4. Ordering & opt-in semantics

- [ ] Standalone providers are interleaved at the position their name
  occupies in `agent_order`/`enabled_tools`, not appended last.
- [ ] A non-empty `enabled_tools` list is authoritative for standalone
  opt-in: the settings UI disables a provider by removing it from
  `enabled_tools` while keeping `agent_order` intact, so treating
  `agent_order` alone as opt-in breaks the toggle.

## 5. Config surfaces (each save path must round-trip)

- [ ] `oct config set tools <names>` accepts standalone names/aliases, not
  just update tools.
- [ ] Interactive picker save preserves standalone entries in `enabled_tools`
  AND preserves their positions in `agent_order` (plain append reorders).
- [ ] `config api` snapshot + `applyConfigUpdate` include standalone
  providers; otherwise a Swift settings save silently disables them.
- [ ] After ANY save path, `oct usage` output order and enabled set match
  what the user configured.

## 6. Alerts

- [ ] Threshold/snooze keys use canonical registry names; resolve aliases and
  legacy tool names via `CanonicalProviderName` on write AND at evaluation
  (`mmx` keys vs `minimax` provider silently never matched).
- [ ] The interactive provider picker derives options from
  `usage.AlertProviderNames()` — hardcoded lists rot when new providers land.

## 7. UI

- [ ] `HexColor` readable on dark AND light terminals; brand near-blacks need
  a lightened substitute (kimi `#16191E` was invisible).
- [ ] Compact-title letter (`CompactLabel`) unique across the registry.

## 8. Docs

- [ ] README claims stay accurate per provider: collection is hybrid (usage
  API where one exists, local records otherwise) — do not promise live API
  reads for providers that parse local files.
- [ ] The command map only advertises what each command really does
  (`oct config` never touched alert thresholds).
- [ ] `usage.md` env-var list and auth-source notes updated.

## 9. Tests & release gate

- [ ] Unit tests for: window mapping, percent conversion, credential
  resolution with the env var UNSET, endpoint override, and one registry
  assertion that the fetcher is keyed by the tool binary name.
- [ ] `GOTOOLCHAIN=auto go test ./...` green; gofmt clean.
- [ ] Live smoke test against the real API (P1 release gate in
  [roadmap_2026-09-12.md](roadmap_2026-09-12.md)) — community-sourced field
  names are a hypothesis, not a verification.

## Related docs

- [usage.md](usage.md): provider auth/env reference to update alongside
- [roadmap_2026-09-12.md](roadmap_2026-09-12.md): phases and the live-API gate
- [local_test.md](local_test.md): endpoint mocking workflow for the smoke test

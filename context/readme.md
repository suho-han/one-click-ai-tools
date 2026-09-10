# context Documentation Index

This index organizes documents under `context/` by purpose. Each file owns one
role; a "Related docs" section at the end of each doc points to adjacent
material.

## User Docs — how to use `oct`

- [usage.md](usage.md): command reference for `update`, `agent-update`, `usage` (one-shot collection, env vars)
- [monitoring.md](monitoring.md): always-on live view with `oct monitor` (continuous refresh, snapshots)
- [usage_alerts.md](usage_alerts.md): threshold-based OS alert configuration and behavior rules

Role boundary: `oct usage` prints one snapshot, `oct monitor` keeps refreshing
a view, `oct usage --notify` pushes alerts when thresholds are crossed.

## Dev & Validation Docs — how to build and validate

- [local_test.md](local_test.md): local run/build/test and endpoint mocking (host-agnostic everyday workflow)
- [macbook_air_smoke_test.md](macbook_air_smoke_test.md): quick post-change smoke test on the primary macOS host (100.73.225.85)
- [platform_e2e_checklist.md](platform_e2e_checklist.md): Linux/macOS/Windows schedule & install E2E checklist (full manual matrix)
- [menubar_helper_operations.md](menubar_helper_operations.md): Swift menubar helper build/install/remote validation

Role boundary: `local_test` is the everyday build/test workflow, the smoke
test is the fast host-specific sanity pass, and the E2E checklist is the full
per-platform manual matrix.

## Internal Reference

- [icons.md](icons.md): provider icon mapping and renderer fallback notes
- [todo.md](todo.md): living list of future work
- [code_review_2026-09-10.md](code_review_2026-09-10.md): dated code-review backlog snapshot (v0.1.5 baseline; `file:line` refs drift)
- [improvement_plan_2026-09-10.md](improvement_plan_2026-09-10.md): execution plan (fix + test design) for the review's Top 10, with review corrections; not yet executed
- [opencode_quota_feature_spec.md](opencode_quota_feature_spec.md): feature spec for a planned `oct quota` command

# context Documentation Index

This index organizes documents under `context/` by purpose. Each file owns one
role; a "Related docs" section at the end of each doc points to adjacent
material.

## User Docs — how to use `oct`

- [usage.md](usage.md): command reference for `update`, `agent-update`, `usage` (one-shot collection, env vars)
- [../docs/usage-json-schema.md](../docs/usage-json-schema.md): stable JSON output contract for `oct usage --json` (for script consumers)
- [monitoring.md](monitoring.md): always-on live view with `oct monitor` (continuous refresh, snapshots)
- [usage_alerts.md](usage_alerts.md): threshold-based OS alert configuration and behavior rules
- [icons.md](icons.md): provider icon mapping and `OCT_ICON_RENDERER` fallback notes

Role boundary: `oct usage` prints one snapshot, `oct monitor` keeps refreshing
a view, `oct usage --notify` pushes alerts when thresholds are crossed.

## Planning & Strategy — what to build next

- [roadmap_2026-09-12.md](roadmap_2026-09-12.md): canonical positioning thesis, workflow command order, phased plan (P0 positioning hygiene → P1 release gate → P2 product depth)
- [competitive_landscape_2026-09-12.md](competitive_landscape_2026-09-12.md): competitor scan + originality/open-source readiness assessment (usage space saturated; update+usage combination unoccupied)
- [opencode_quota_feature_spec.md](opencode_quota_feature_spec.md): feature spec for a planned `oct quota` command (roadmap P2)
- [todo.md](todo.md): flat backlog of future work + usage-provider endpoint research
- [usage_provider_checklist.md](usage_provider_checklist.md): pre-merge checklist for adding/changing a usage provider (each item traces to a real review finding)

Role boundary: `roadmap` decides order and priorities, `competitive_landscape`
holds the evidence behind the positioning, `quota spec` is the per-feature
design, `todo` is the unstructured backlog feeding all of them.

## Dev & Validation Docs — how to build and validate

- [local_test.md](local_test.md): local run/build/test and endpoint mocking (host-agnostic everyday workflow)
- [macbook_air_smoke_test.md](macbook_air_smoke_test.md): quick post-change smoke test on the primary macOS host (100.73.225.85)
- [platform_e2e_checklist.md](platform_e2e_checklist.md): Linux/macOS/Windows schedule & install E2E checklist (full manual matrix)
- [menubar_helper_operations.md](menubar_helper_operations.md): Swift menubar helper build/install/remote validation

Role boundary: `local_test` is the everyday build/test workflow, the smoke
test is the fast host-specific sanity pass, and the E2E checklist is the full
per-platform manual matrix.

## Audit Snapshots — dated, historical

Point-in-time review snapshots; their `file:line` references drift with new
commits. Do not edit them to track ongoing work — write a new dated doc
instead.

- [code_review_2026-09-10.md](code_review_2026-09-10.md): first audit pass (top-10 backlog; v0.1.5 baseline)
- [improvement_plan_2026-09-10.md](improvement_plan_2026-09-10.md): execution plan for the first pass's Top 10 (executed; still-open 4-B / 4-C release items)
- [code_review_2026-09-11.md](code_review_2026-09-11.md): second audit pass (error handling + inefficiencies); fixes landed and pushed

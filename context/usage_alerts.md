# Usage Alerts Guide

When `oct usage --notify` or `usage_alert_enabled=true` is active, `oct` sends OS alerts on threshold breaches.

## Core configuration

Example `~/.oct/config.yaml`:

```yaml
usage_alert_enabled: true
usage_alert_threshold_percent: 80
usage_alert_critical_percent: 98
usage_alert_cooldown_minutes: 360
# Quiet timer: alerts stay suppressed until this instant (empty = off).
# Armed via the preset choices off/1h/2h/4h/6h/12h, not edited by hand.
usage_alert_quiet_until: "2026-09-26T22:00:00+09:00"

usage_alert_thresholds:
  default: 80
  "5h": 85
  "7d": 90

usage_alert_provider_thresholds:
  codex:
    default: 85
    "5h": 90
  cursor:
    "5h": 88
  opencode:
    default: 87
```

Threshold precedence:
1. `provider + window`
2. `provider + default`
3. `global + window`
4. `usage_alert_threshold_percent` (global default)

## CLI

Run `oct alert` without a subcommand for the interactive arrow/key-based alert setup.
Use `oct config` for provider and usage-display configuration; it does not configure notifications.
The advanced alert controls remain under `oct alert config ...`, including provider-specific thresholds, while `oct alert test ...` evaluates synthetic input and `oct alert snooze ...` manages snoozes.
In macOS menubar Settings, common alert controls are under Configuration, where the former General tab is merged.

```bash
oct alert

oct alert config show

oct alert config set enabled true
oct alert config set cooldown_minutes 120
oct alert config set threshold_percent 85
oct alert config set critical_percent 98
oct alert config set quiet 2h          # choices: off, 1h, 2h, 4h, 6h, 12h

oct alert config set threshold.5h 90
oct alert config set threshold.7d 92

oct alert config set provider.codex.5h 94
oct alert config set provider.codex.default 88
oct alert config set provider.cursor.5h 93
oct alert config set provider.opencode.default 87

oct alert config set-provider-threshold 5h 93 --provider cursor

oct alert test --provider codex --window 5h --value 96

oct alert snooze set --duration 2h
oct alert snooze set --duration 1h --provider codex --window 5h
oct alert snooze show
oct alert snooze clear --provider codex --window 5h
```

## Behavior rules

- Priority labels:
  - `value >= critical_percent` -> `CRITICAL`
  - `threshold <= value < critical_percent` -> `HIGH`
- Duplicate alerts are suppressed during cooldown windows
- Escalation can still alert during cooldown if threshold level increases
- While the quiet timer is armed, only `CRITICAL` passes
- The quiet timer is a one-shot mute (off/1h/2h/4h/6h/12h) armed from the moment it is set; it expires on its own
- `CRITICAL` overrides snooze
- State file: `~/.oct/state/usage-alert-state.json`

## Related docs

- [monitoring.md](monitoring.md): continuous live view without alerting
- [usage.md](usage.md): `oct usage --notify` trigger and provider details

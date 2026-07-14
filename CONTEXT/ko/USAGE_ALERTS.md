# 사용량 알림 가이드

`oct usage --notify` 또는 `usage_alert_enabled=true`일 때, 임계치 초과 시 OS 알림을 보냅니다.

## 핵심 설정

`~/.oct/config.yaml` 예시:

```yaml
usage_alert_enabled: true
usage_alert_threshold_percent: 80
usage_alert_critical_percent: 98
usage_alert_cooldown_minutes: 360
usage_alert_quiet_hours: "00:00-08:00"
usage_alert_timezone: "Asia/Seoul"

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

설정 우선순위:
1. `provider + window`
2. `provider + default`
3. `global + window`
4. `usage_alert_threshold_percent` (global default)

## CLI

```bash
oct alert config show

oct alert config set enabled true
oct alert config set cooldown_minutes 120
oct alert config set threshold_percent 85
oct alert config set critical_percent 98
oct alert config set quiet_hours 00:00-08:00
oct alert config set timezone Asia/Seoul

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

## 동작 규칙

- 우선순위 라벨:
  - `value >= critical_percent` -> `CRITICAL`
  - `threshold <= value < critical_percent` -> `HIGH`
- 쿨다운 중 중복 알림 억제
- 더 높은 구간으로 상승 시 쿨다운 중이라도 알림 가능
- quiet hours에는 `CRITICAL`만 통과
- snooze 중에도 `CRITICAL`은 override
- 상태 파일: `~/.oct/state/usage-alert-state.json`

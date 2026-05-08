# Usage Alerts (P6)

`oct usage --notify`는 사용량 임계치 초과 시 OS 알림을 보냅니다.

## 핵심 설정 키

`~/.oct/config.yaml` 예시:

```yaml
usage_alert_enabled: true
usage_alert_threshold_percent: 80
usage_alert_critical_percent: 98
usage_alert_cooldown_minutes: 360
usage_alert_quiet_hours: "00:00-08:00"
usage_alert_timezone: "Asia/Seoul"

# global threshold override
usage_alert_thresholds:
  default: 80
  "5h": 85
  "7d": 90

# provider-specific overrides
usage_alert_provider_thresholds:
  codex:
    default: 85
    "5h": 90
  cursor:
    "5h": 88
```

우선순위:
1. provider+window
2. provider default
3. global window
4. global default / usage_alert_threshold_percent

## CLI

```bash
# 설정 보기
oct alert config show

# 간단 설정 변경
oct alert config set enabled true
oct alert config set cooldown_minutes 120
oct alert config set threshold_percent 85
oct alert config set critical_percent 98
oct alert config set quiet_hours 00:00-08:00
oct alert config set timezone Asia/Seoul

# 알림 로직 테스트
oct alert test --provider codex --window 5h --value 96

# 스누즈
oct alert snooze set --duration 2h
oct alert snooze set --duration 1h --provider codex --window 5h
oct alert snooze show
oct alert snooze clear --provider codex --window 5h
```

## 동작 규칙

- 쿨다운 내 중복 알림 방지
- 더 높은 임계치로 상승 시(예: 85→95) 쿨다운 내라도 알림
- Quiet hours 동안은 95% 미만 알림 억제
- snooze 동안은 알림 억제(단, critical_percent 이상은 override)
- 상태 파일: `~/.oct/state/usage-alert-state.json`

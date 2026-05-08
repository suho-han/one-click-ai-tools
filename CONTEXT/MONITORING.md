# Always-on Monitoring (P7)

`oct monitor`는 리눅스/윈도우 공통으로 터미널 상시 모니터링 화면을 제공합니다.

## 기본 사용

```bash
# 30초 주기
oct monitor

# 5초 주기
oct monitor --interval 5s

# 1회만 실행
oct monitor --once

# 사용량 높은 순 상위 5개만 간소 출력
oct monitor --once --sort-by used --desc --top 5 --compact
```

## 출력 내용

- provider별 5h / 7d / used / severity(OK/WARN/CRIT) / status
- 갱신 시각
- 메시지 요약

정렬 옵션:
- `--sort-by provider|used|5h|7d`
- `--desc`
- `--top N`
- `--compact`

## 상태 스냅샷

매 갱신마다 JSON 스냅샷 저장:

- 기본 경로: `~/.oct/state/usage-latest.json`
- 커스텀 경로: `oct monitor --state-path /tmp/oct-usage.json`

## 상시 운영 팁

### Linux
- `tmux` 세션에서 실행 권장

```bash
tmux new -s oct-monitor 'oct monitor --interval 10s'
```

### Windows (PowerShell)
- 별도 창에서 실행 후 고정

```powershell
oct monitor --interval 10s
```

## 향후 확장

이 스냅샷 JSON을 읽어 윈도우 트레이/작업표시줄 UI로 확장 가능.

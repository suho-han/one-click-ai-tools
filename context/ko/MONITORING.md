# 상시 모니터링 가이드

`oct monitor`는 provider별 사용량을 주기적으로 갱신해 보여줍니다.

## 기본 사용

```bash
oct monitor
oct monitor --interval 5s
oct monitor --once
oct monitor --once --sort-by used --desc --top 5 --compact
```

## 주요 옵션

- `--interval`: 갱신 주기 (기본 30초)
- `--once`: 1회 실행 후 종료
- `--sort-by provider|used|5h|7d`: 정렬 키
- `--desc`: 내림차순 정렬
- `--top N`: 상위 N개만 출력
- `--compact`: 간소 출력
- `--state-path`: 스냅샷 저장 경로 지정

## 출력/스냅샷

- 컬럼: `provider`, `5h`, `7d`, `sev`, `status` (+ 기본 모드에서 `used`, `limit`, `message`)
- 스냅샷 기본 경로: `~/.oct/state/usage-latest.json`
- `usage_display_mode=remaining`이면 잔여량(100-used) 기준으로 표시

## 운영 팁

Linux:
```bash
tmux new -s oct-monitor 'oct monitor --interval 10s'
```

Windows (PowerShell):
```powershell
oct monitor --interval 10s
```

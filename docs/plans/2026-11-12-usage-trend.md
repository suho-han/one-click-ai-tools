# Usage Trend & Exhaustion ETA (2026-11-12)

Status: planned, not implemented. Target: next release batch (deferred release).

## 목적

`oct usage`는 지금 1회 스냅샷만 보여준다. 같은 조회를 반복하면 "지금 얼마 남았나"는
알 수 있지만 "이 속도면 언제 바닥나는가"는 알 수 없다. CodexBar/caut 계열 대비 CLI
차별점이 비어 있는 지점이고, 2026-09-25 `oct quota` 제거(`94697c4`) 이후 리셋
카운트다운/비용 시뮬레이터도 없다.

이 배치는 **로컬 이력 + 소진 예측**만 다룬다. menubar(Swift)와 원격 통계 서버는
범위 밖이다.

## 설계

### 저장

- 경로: `~/.oct/state/usage-history.jsonl` (`internal/usage`의 기존 state 디렉터리 규칙 재사용)
- 한 줄 = 한 (provider, window) 샘플:

```json
{"ts":"2026-11-12T07:00:00Z","provider":"codex","window":"5h","unit":"percent","used":42,"limit":100}
```

- 샘플 소스: `UsageResult.Period`(창 없음) + `Buckets`/`BucketResets`의 각 창
  (`5h`, `7d` 등). `Unit`이 `percent`가 아니거나 `Used`/`Limit`가 숫자로 해석되지
  않는 행은 기록하지 않는다.
- 기록 시점: `oct usage` 라이브 조회 성공 직후, 그리고 `oct monitor` 사이클.
  `--from-snapshot` 렌더링에서는 기록하지 않는다(과거 스냅샷 재기록 방지).
- 끄기: `--no-history` 플래그 또는 `OCT_USAGE_HISTORY=0`.

### 보존

- 30일 초과 샘플 삭제, 최대 20,000줄. 초과 시 오래된 줄부터 절단.
- 파일은 0600, 디렉터리 0700 (`fsutil.WriteFileAtomic` 계열 사용).
- 파싱 실패 줄은 건너뛰고 나머지로 계속 진행(단일 손상 줄이 전체를 막지 않게).
- 동시 실행 보호: append는 O_APPEND 한 줄 쓰기, 절단은 임시 파일 교체.

### 계산

- 입력: 동일 `(provider, window)`의 최근 6시간 샘플, 최소 2개.
- 사용 증가율(%/h) = (최신 사용% − 최초 사용%) / 경과시간. 음수(리셋/창 전환)면
  해당 구간 폐기 후 재계산, 재계산 불가면 예측 없음.
- 잔여% = 100 − 최신 사용%. `eta_at = now + 잔여% / rate`.
- rate가 0이면 ETA 없음. ETA가 현재 창의 `BucketResets` 시각보다 뒤면 "창 리셋
  전 소진 없음"으로 표시 대상에서 제외(또는 `eta_after_reset` 표시).

### 노출

- `oct usage --json`: 각 결과에 `history` 객체 추가
  `{"burn_rate_per_hour":1.8,"eta_at":"2026-11-12T15:04:05Z","samples":7}` (없으면 생략).
- 표 출력: `ETA` 컬럼 추가. 값 없으면 `-`. `--no-trend`로 컬럼/필드 생략.
- 상태라인 포맷(waybar/polybar/swiftbar)·compact 출력은 변경하지 않는다.

## 테스트

- `internal/usage/history_test.go`
  - append 후 JSONL 라운드트립, 0600 권한
  - 보존: 30일 초과·20k줄 초과 절단
  - 손상 줄 1개 포함 시 나머지 샘플 유지
  - burn: 샘플 2개 선형, 사용 0(rate 0), 리셋(감소) 구간 폐기, 샘플 1개 → 예측 없음
- `cmd/usage_test.go`: `--json`에 `history` 필드, `--no-trend`/`--no-history` 동작

## 검증

- `GOTOOLCHAIN=auto go test ./...`
- `go build -o oct main.go` 후 임시 `HOME`으로 `oct usage --json` 2회 실행해 히스토리 파일 생성 확인

## 범위 제외

- menubar 표시, 원격 통계 서버 업로드, 비용(USD) 환산, 알림 규칙 연동

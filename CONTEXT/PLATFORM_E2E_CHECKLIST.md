# Platform E2E Checklist (Linux / Windows)

## 공통 사전 준비
- `oct` 바이너리 PATH 등록 확인
- `oct schedule status` 실행 가능 여부 확인
- 로그 경로 쓰기 권한 확인

## Linux (cron)
1. `oct schedule enable --interval daily --hour 3`
2. `crontab -l`에 `oct agent-update` 엔트리 1개만 존재 확인
3. `~/.oct/logs/schedule.log` 생성 확인
4. `oct schedule status`가 `enabled` 반환 확인
5. `oct schedule disable`
6. `crontab -l`에서 엔트리 제거 확인
7. `oct schedule status`가 `disabled` 반환 확인

## Windows (Task Scheduler)
1. PowerShell 관리자 권한으로 `oct schedule enable --interval daily --hour 3`
2. `schtasks /Query /TN OneClickToolsUpdate` 성공 확인
3. Task action이 `oct agent-update`인지 확인
4. `oct schedule status`가 `enabled` 반환 확인
5. `oct schedule disable`
6. `schtasks /Query /TN OneClickToolsUpdate` 실패(없음) 확인
7. `oct schedule status`가 `disabled` 반환 확인

## CI smoke matrix
- GitHub Actions: `.github/workflows/smoke-matrix.yml`
- 대상 OS:
  - `ubuntu-latest`
  - `windows-latest`
- 검증 항목:
  - `go test ./...`
  - `go build ./...`

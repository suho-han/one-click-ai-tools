# 플랫폼 E2E 체크리스트 (Linux / Windows)

## 공통

- `oct` 실행 파일 PATH 확인
- `oct schedule`로 상태 출력 확인
- 로그/상태 파일 경로 쓰기 권한 확인

## Linux (cron)

1. `oct schedule enable --interval daily --hour 3`
2. `crontab -l`에서 `oct agent-update` 엔트리 1개 확인
3. `oct schedule` 상태가 `enabled`인지 확인
4. `oct schedule disable`
5. `crontab -l`에서 엔트리 제거 확인

## Windows (Task Scheduler)

1. `oct schedule enable --interval daily --hour 3`
2. `schtasks /Query /TN OneClickToolsUpdate` 성공 확인
3. 작업 Action이 `oct agent-update`인지 확인
4. `oct schedule` 상태가 `enabled`인지 확인
5. `oct schedule disable`
6. `schtasks /Query /TN OneClickToolsUpdate`가 실패(없음)하는지 확인

## CI 스모크

- OS: `ubuntu-latest`, `windows-latest`
- 항목: `go test ./...`, `go build ./...`

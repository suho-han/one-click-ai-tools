# 플랫폼 E2E 체크리스트 (Linux / macOS / Windows)

## 목적

`schedule enable/disable`와 설치 시 `session-refresh` 연결 흐름이 실제 운영 환경에서 올바른 실행 파일, 올바른 task 이름, 올바른 로그 경로를 사용하도록 검증한다.

## 공통 사전 확인

- `oct` 실행 파일 위치 확인
  - `command -v oct` (Linux/macOS)
  - `where oct` (Windows)
- 버전 및 기본 실행 확인
  - `oct --version`
  - `oct schedule --task agent-update`
  - `oct schedule --task session-refresh`
- 홈 디렉터리 및 로그 경로 쓰기 권한 확인
  - `~/.oct/`
  - `~/.oct/logs/`
- 기존 스케줄 엔트리 정리 여부 확인
  - `agent-update`
  - `session-refresh`

## 설치(postinstall) 검증

### 공통

1. 새 HOME 또는 테스트 계정에서 깨끗한 상태 준비
2. `npm install -g one-click-tools` 또는 동등한 설치 흐름 실행
3. 설치 직후 `~/.oct/config.yaml` 생성 여부 확인
4. 다음 기본값 확인
   - `session_refresh_enabled: false`
   - `session_refresh_interval: daily`
   - `session_refresh_hour: 9`

### 비대화형(non-interactive)

1. 비TTY/CI 환경에서 설치 실행
2. `Enable periodic token-free session refresh?` 프롬프트가 뜨지 않는지 확인
3. `session_refresh_enabled: false` 유지 확인
4. scheduler 엔트리가 자동 생성되지 않는지 확인

### 강제 enable

1. `OCT_INSTALL_ENABLE_SESSION_REFRESH=yes`로 설치 실행
2. `session_refresh_enabled: true` 저장 확인
3. `session-refresh` 스케줄이 자동 등록되는지 확인
4. 등록된 엔트리가 **현재 설치된 oct binary 경로**를 가리키는지 확인

### 대화형(interactive)

1. TTY 환경에서 설치 실행
2. `Enable periodic token-free session refresh? [y/N]:` 표시 확인
3. `y` 응답 시 `session_refresh_enabled: true` 저장 확인
4. 자동 등록된 스케줄 엔트리가 `session-refresh` task를 가리키는지 확인
5. 등록된 엔트리가 PATH 상 다른 `oct`가 아니라 **방금 설치한 oct** 경로를 가리키는지 확인

## Linux (cron)

### agent-update

1. `oct schedule enable --task agent-update --interval daily --hour 3`
2. `crontab -l`에서 `# oct-managed:agent-update` 엔트리 1개 확인
3. 엔트리 명령이 `oct agent-update`인지 확인
4. 로그 경로가 `~/.oct/logs/agent-update.log`인지 확인
5. `oct schedule --task agent-update` 상태가 `enabled`인지 확인
6. 동일 명령 재실행 후 중복 엔트리가 생기지 않는지 확인
7. `oct schedule disable --task agent-update`
8. `crontab -l`에서 `agent-update` 엔트리 제거 확인

### session-refresh

1. `oct schedule enable --task session-refresh --interval daily --hour 9`
2. `crontab -l`에서 `# oct-managed:session-refresh` 엔트리 1개 확인
3. 엔트리 명령이 `oct session-refresh`인지 확인
4. 로그 경로가 `~/.oct/logs/session-refresh.log`인지 확인
5. 엔트리에 기록된 binary path가 현재 실행 중인 `oct`와 일치하는지 확인
6. `oct schedule --task session-refresh` 상태가 `enabled`인지 확인
7. 동일 명령 재실행 후 중복 엔트리가 생기지 않는지 확인
8. `oct schedule disable --task session-refresh`
9. `crontab -l`에서 `session-refresh` 엔트리 제거 확인

## macOS (launchctl)

### agent-update

1. `oct schedule enable --task agent-update --interval daily --hour 3`
2. `~/Library/LaunchAgents/com.oct.agent-update.plist` 생성 확인
3. `ProgramArguments`가 `[<oct binary>, "agent-update"]`인지 확인
4. `StandardOutPath` / `StandardErrorPath`가 `~/.oct/logs/agent-update.log`인지 확인
5. `launchctl list | grep com.oct.agent-update` 또는 동등 명령으로 로드 확인
6. `oct schedule --task agent-update` 상태가 `enabled`인지 확인
7. `oct schedule disable --task agent-update`
8. plist 제거 및 unload 확인

### session-refresh

1. `oct schedule enable --task session-refresh --interval daily --hour 9`
2. `~/Library/LaunchAgents/com.oct.session-refresh.plist` 생성 확인
3. `ProgramArguments`가 `[<oct binary>, "session-refresh"]`인지 확인
4. plist가 PATH 상 다른 `oct`가 아니라 현재 binary 경로를 가리키는지 확인
5. `StandardOutPath` / `StandardErrorPath`가 `~/.oct/logs/session-refresh.log`인지 확인
6. `launchctl list | grep com.oct.session-refresh` 또는 동등 명령으로 로드 확인
7. `oct schedule --task session-refresh` 상태가 `enabled`인지 확인
8. `oct schedule disable --task session-refresh`
9. plist 제거 및 unload 확인

## Windows (Task Scheduler)

### agent-update

1. `oct schedule enable --task agent-update --interval daily --hour 3`
2. `schtasks /Query /TN OneClickToolsUpdate` 성공 확인
3. 작업 Action이 `<oct binary> agent-update`인지 확인
4. 로그/출력 경로 정책이 문서와 일치하는지 확인
5. `oct schedule --task agent-update` 상태가 `enabled`인지 확인
6. `oct schedule disable --task agent-update`
7. `schtasks /Query /TN OneClickToolsUpdate`가 실패(없음)하는지 확인

### session-refresh

1. `oct schedule enable --task session-refresh --interval daily --hour 9`
2. `schtasks /Query /TN OneClickToolsSessionRefresh` 성공 확인
3. 작업 Action이 `<oct binary> session-refresh`인지 확인
4. 실행 binary 경로 quoting이 공백 경로에서도 안전한지 확인
5. PATH 상 다른 `oct`가 아니라 현재 binary 경로를 가리키는지 확인
6. `oct schedule --task session-refresh` 상태가 `enabled`인지 확인
7. `oct schedule disable --task session-refresh`
8. `schtasks /Query /TN OneClickToolsSessionRefresh`가 실패(없음)하는지 확인

## 최소 회귀 테스트

- `GOTOOLCHAIN=auto go test ./internal/schedule -v`
- `GOTOOLCHAIN=auto go test ./...`
- `go build -o oct main.go`

## 수동 검증 완료 기준

- install-time prompt / env 강제 enable / non-interactive 기본값이 모두 기대대로 동작
- `agent-update`, `session-refresh`가 플랫폼별로 서로 다른 엔트리/작업 이름으로 안전하게 공존
- 등록 엔트리가 항상 **현재 oct binary**를 가리킴
- enable 재실행 시 중복 엔트리가 생기지 않음
- disable 후 엔트리가 완전히 제거됨

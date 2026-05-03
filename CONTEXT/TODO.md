# oct (v0.4.2) TODO & Known Issues - STATUS: ROLLED OVER

## 🛠 남은 과제 (Remaining Tasks)

### 1. 시스템 및 설정 고도화

- [ ] **추가 패키지 매니저**: `cargo`, `go install`, `pip` 등 확장 지원
- [ ] **Windows/Linux 최종 검증**: 실제 타겟 환경에서의 스케줄링 동작 및 경로 처리 재확인

### 2. 지원 도구 및 사용량 추적 확장

- [ ] **Cursor API/CLI**: Cursor 에디터의 모델별 사용량 조회 및 설정 동기화 탐색
- [ ] **기타 AI 에이전트 확장**: 다양한 자율형 에이전트 CLI의 업데이트 및 사용량 지원 확대

### 3. 플랫폼 확장 (UI/UX)

- [ ] **macOS Menu Bar (상단바)**: `systray` 등을 활용하여 상단바에서 실시간 AI 사용량(Quota) 확인 기능
- [ ] **macOS Widgets**: 데스크탑 위젯을 통한 주요 모델 잔여 쿼타 시각화
- [ ] **알림 서비스**: 사용량이 특정 임계치(예: 90%)에 도달했을 때 OS 알림 발송

---

## ✅ v0.4.0에서 완료된 사항 (Completed in v0.4.0)

### 1. 시스템 및 설정 고도화

- [x] **에러 핸들링 고도화**: 네트워크 장애 시 재시도 로직 및 상세 에러 가이드 추가 (`internal/netclient`)
- [x] **에이전트 순서 커스터마이징**: 사용자가 선호하는 에이전트 순서를 설정(config)하고, 이를 `usage`, `agent-update`, `config` 등 모든 출력 및 실행 로직에 반영
- [x] **로딩 화면 출력**: `usage` 사용 시 로딩화면 출력

### 2. 지원 도구 및 사용량 추적 확장

- [x] **시간대별 사용량 구분 (5h + 1w)**: 단기(5시간 단위 등) 및 장기(1주일 단위 등) 쿼타 초기화 주기에 따른 상세 사용량 시각화 구현 (Claude 적용 완료)
- [x] **Token-less 사용량 조회**: API 토큰 없이도 로컬 세션이나 CLI Fallback을 통해 **GitHub Copilot** 및 **Gemini** 사용량을 가져오는 로직 구현

### 3. 사용량 조회 (usage)

- [x] **GitHub Copilot**: API 연동 로직 구현 완료 (`copilot.go`)
- [x] **OpenAI Codex**: 로컬 세션 로그 파싱 로직 구현 완료 (`codex.go`)
- [x] **Gemini**: 실제 API 호출을 통한 Quota 데이터 수신 기초 구현 (`gemini.go`)
- [x] **Claude**: 실제 API 호출을 통한 Usage 데이터 수신 기초 구현 (`claude.go`)
- [x] **병렬 조회 및 로딩 화면**: `errgroup`을 통한 병렬 Fetch 및 `bubbletea` 로딩 스피너 구현

### 4. 업데이트 (update)

- [x] **패키지 매니저 감지**: `npm` 외에 `brew`, `pnpm`, `yarn` 지원 및 자동 감지 로직 구현 (`manager.go`)
- [x] **UI 개선**: 진행 상황 표시 및 소요 시간 출력 기능 추가

### 5. 배포 (deployment)

- [x] **postinstall.js**: OS/Arch 기반 GitHub Release 바이너리 자동 다운로드 로직 구현
- [x] **GoReleaser CI**: GitHub Actions 연동 설정 완료 (`.github/workflows/release.yml`)

### 6. 테스트 보완

- [x] `cmd/` 패키지 통합 테스트 추가 (`root_test.go`)
- [x] `internal/usage/` 유닛 테스트 및 Mock 서버 테스트 추가 (`usage_test.go`)

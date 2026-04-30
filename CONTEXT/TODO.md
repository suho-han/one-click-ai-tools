# oct (v0.3.0) TODO & Known Issues - STATUS: UPDATED

## ✅ 완료된 사항 (Completed)

### 1. 사용량 조회 (usage)
- [x] **GitHub Copilot**: API 연동 로직 구현 완료 (`copilot.go`)
- [x] **OpenAI Codex**: 로컬 세션 로그 파싱 로직 구현 완료 (`codex.go`)
- [x] **Gemini**: 실제 API 호출을 통한 Quota 데이터 수신 기초 구현 (`gemini.go`)
- [x] **Claude**: 실제 API 호출을 통한 Usage 데이터 수신 기초 구현 (`claude.go`)

### 2. 업데이트 (update)
- [x] **패키지 매니저 감지**: `npm` 외에 `brew`, `pnpm`, `yarn` 지원 및 자동 감지 로직 구현 (`manager.go`)
- [x] **UI 개선**: 진행 상황 표시 및 소요 시간 출력 기능 추가

### 3. 배포 (deployment)
- [x] **postinstall.js**: OS/Arch 기반 GitHub Release 바이너리 자동 다운로드 로직 구현
- [x] **GoReleaser CI**: GitHub Actions 연동 설정 완료 (`.github/workflows/release.yml`)

### 4. 테스트 보완
- [x] `cmd/` 패키지 통합 테스트 추가 (`root_test.go`)
- [x] `internal/usage/` 유닛 테스트 및 Mock 서버 테스트 추가 (`usage_test.go`)

---

## 🛠 남은 과제 (Remaining Tasks)
- [ ] **에러 핸들링 고도화**: 네트워크 장애 시 재시도 로직 추가
- [ ] **추가 패키지 매니저**: `cargo`, `go install` 등 필요한 경우 확장
- [ ] **Windows/Linux 최종 검증**: 실제 타겟 환경에서의 스케줄링 동작 재확인

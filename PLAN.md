# PLAN - Go Migration (v0.3.0) - STATUS: COMPLETED

## 마이그레이션 목표
현재 약 2,500줄의 Bash 스크립트를 Go로 재작성하여 성능, 안정성, 유지보수성 및 **Windows 지원**을 확보한다. (완료)

- **성능**: 쉘 스크립트 대비 압축적인 실행 속도와 병렬 처리(Goroutine) 활용
- **안정성**: 타입 시스템 도입으로 런타임 에러 방지
- **확장성**: Windows(Task Scheduler) 지원 추가
- **배포**: 단일 바이너리 배포 (사용자 환경의 Bash 버전/의존성 무관)

---

## 아키텍처 (Go)

- **CLI Framework**: \`github.com/spf13/cobra\`
- **Config Management**: \`github.com/spf13/viper\`
- **Concurrency**: \`sync/errgroup\`을 이용한 여러 AI 도구 동시 업데이트
- **OS Abstraction**:
    - macOS: \`launchd\`
    - Linux: \`cron\` (placeholder implemented)
    - Windows: \`Task Scheduler (schtasks)\` (implemented)

---

## Phase 1: Core Foundation [DONE]

### 1. 프로젝트 초기화
- \`go mod init github.com/suho-han/one-click-tools\`
- Cobra CLI 구조 생성 (\`cmd/root.go\`, \`cmd/agent_update.go\` 등)

### 2. 설정 시스템 (Viper)
- \`~/.oct/config.yaml\` 사용
- 기존 \`.sh\` 기반 설정을 YAML로 자동 마이그레이션하는 로직 구현

---

## Phase 2: Feature Migration [DONE]

### 1. \`oct agent-update\` (핵심 기능)
- 각 OS별 패키지 매니저(npm) 실행 로직 이관
- **개선**: 여러 도구 업데이트 시 Goroutine을 사용하여 병렬 실행 (속도 향상)

### 2. \`oct usage\` (사용량 조회)
- 복잡한 \`usage-report.sh\`의 로직을 Go Struct로 구조화
- Gemini OAuth 기반 유저 정보 확인 로직 기초 구현

### 3. \`oct schedule\` (스케줄링)
- OS별 추상화 인터페이스 구현 (macOS \`launchd\` 연동 완료)

---

## Phase 3: Windows 지원 [DONE]

- **경로 처리**: \`path/filepath\`를 사용하여 \`/\`와 \`\\\` 구분 처리
- **스케줄링**: \`schtasks\`를 이용한 윈도우 작업 스케줄러 등록 기능 구현 완료

---

## Phase 4: 배포 전략 [DONE]

### 1. GoReleaser 설정
- \`.goreleaser.yaml\` 작성 (darwin, linux, windows 빌드 자동화)

### 2. NPM Wrapper
- (추후 작업) v0.3.0 배포 시 기존 NPM 배포 프로세스에 Go 바이너리 포함 작업 필요

---

## 구현 결과물

1. **Go 프로젝트 구성 완료**
2. **핵심 기능 (agent-update) 병렬화 완료**
3. **설정 자동 마이그레이션 완료**
4. **macOS/Windows 스케줄링 로직 구현 완료**

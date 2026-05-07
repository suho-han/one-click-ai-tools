# 로컬 테스트 가이드 (Go 버전)

## 0. Go 설치하기

이 프로젝트를 로컬에서 실행하고 빌드하려면 Go 언어가 설치되어 있어야 합니다.

**macOS (현재 환경)**

```bash
# Homebrew를 사용한 설치 (권장)
brew install go
```

**Ubuntu (Linux)**

```bash
# APT 패키지 매니저를 사용한 설치
sudo apt update
sudo apt install golang-go
```

또는 [Go 공식 웹사이트(go.dev/dl/)](https://go.dev/dl/)에서 각 운영체제에 맞는 설치 패키지를 다운로드하여 설치할 수 있습니다.

설치 확인:

```bash
go version
```

## 1. `go run`으로 즉시 실행 (가장 빠름)

빌드 단계 없이 소스 코드를 직접 실행하여 동작을 확인한다.

```bash
# 도움말 출력
go run main.go help

# 특정 도구 사용량 조회 (JSON 출력)
go run main.go usage --json

# AI 도구 업데이트 (실제 brew/npm/pnpm 등 호출됨 — 주의)
go run main.go agent-update

# 설정 목록 확인
go run main.go config list
```

## 2. 바이너리 빌드 및 실행

실제 배포될 바이너리 형태로 빌드하여 테스트한다.

```bash
# 바이너리 빌드 (프로젝트 루트에 'oct' 생성)
go build -o oct main.go

# 빌드된 바이너리 실행
./oct help
./oct usage
```

## 3. npm link로 `oct` 명령어 테스트

Go로 빌드된 바이너리를 `npm link`를 통해 전역 명령어로 등록하여 실제 설치 환경과 동일하게 테스트한다.
`package.json`의 `bin` 설정이 `scripts/oct-wrapper.js`를 가리키고 있으므로, 이 래퍼가 로컬에서 빌드된 바이너리나 소스를 실행하게 된다.

```bash
# 1. 먼저 Go 바이너리 빌드
go build -o oct main.go

# 2. npm link 실행
npm link

# 3. 이후 어디서나 oct 명령어로 테스트
oct help
oct usage

# 테스트 완료 후 해제
npm unlink -g one-click-tools
```

## 4. 유닛 및 통합 테스트 실행

Go의 테스트 도구를 사용하여 코드의 논리적 무결성을 검증한다.

```bash
# 전체 테스트 실행
go test ./...

# 특정 패키지 테스트 (예: usage)
go test ./internal/usage/... -v

# 테스트 커버리지 확인
go test -cover ./...
```

## 5. 환경 변수를 이용한 API Mocking 테스트

`usage` 커맨드 등을 테스트할 때 실제 API 호출 대신 환경 변수를 조작하여 동작을 확인할 수 있다.

```bash
# Mock 엔드포인트 설정 (예시)
OCT_CLAUDE_USAGE_ENDPOINT="http://localhost:8080/usage" \
  go run main.go usage
```

## 6. Windows 검증 가이드

Windows 검증은 `WSL`이 아니라 실제 `Windows 11 + PowerShell` 환경 기준으로 진행한다. 이 프로젝트의 Windows 스케줄링은 `schtasks`를 직접 호출하므로, 실제 `Task Scheduler` 등록과 실행까지 확인해야 한다.

### 6-1. 검증 환경

- 권장 환경: `Windows 11`, `PowerShell 7`
- 확인 대상: `oct.exe`, `npm install -g` 설치 흐름, `Task Scheduler`
- 사전 확인:
  - `go version`
  - `node -v`
  - `npm -v`
  - 필요 시 `pnpm -v`, `yarn -v`

### 6-2. Windows에서 직접 빌드

PowerShell에서 프로젝트 루트로 이동한 뒤 실행한다.

```powershell
go build -o oct.exe main.go
.\oct.exe help
.\oct.exe usage --json
```

기본 스모크 테스트 통과 조건:

- `help`가 정상 출력된다.
- `usage --json`이 비정상 종료 없이 실행된다.
- 설정 관련 커맨드가 Windows 경로에서 오류 없이 동작한다.

### 6-3. npm 설치 흐름 검증

`postinstall.js`는 Windows에서 GitHub Release의 `.zip` 아카이브를 내려받고 `powershell Expand-Archive`로 `bin/oct.exe`를 설치한다. 따라서 다운로드 성공 경로와 fallback 빌드 경로를 둘 다 확인하는 것이 좋다.

```powershell
npm install
npx oct help
```

확인 포인트:

- `bin\oct.exe`가 생성되는지
- 압축 해제 후 실행 파일을 정상 탐지하는지
- 다운로드 실패 시 `npm run build` fallback이 동작하는지
- PowerShell이 없는 환경이 아닌지

### 6-4. 스케줄링 검증

현재 Windows 구현은 `schtasks`로 `OneClickToolsUpdate` 작업을 만들고, 실행 명령은 `oct agent-update` 형태로 등록한다.

```powershell
.\oct.exe schedule
.\oct.exe schedule enable --interval daily --hour 9
.\oct.exe schedule
schtasks /Query /TN OneClickToolsUpdate
```

주요 검증 항목:

- `Schedule enabled (daily, 09:00)`가 출력되는지
- `schtasks /Query`에서 `OneClickToolsUpdate` 작업이 보이는지
- 등록된 작업의 실행 경로가 올바른지
- `oct.exe` 경로에 공백이 있을 때도 작업 등록이 깨지지 않는지
- 로그인 사용자 기준으로 작업이 실행 가능한지

추가로 실제 실행 검증도 진행한다.

```powershell
schtasks /Run /TN OneClickToolsUpdate
schtasks /Query /TN OneClickToolsUpdate /V /FO LIST
```

실행 검증 시 확인할 내용:

- 작업이 수동 실행되는지
- `agent-update`가 실행되면서 경로 문제나 권한 오류가 없는지
- 필요 시 로그/출력으로 실패 원인을 추적할 수 있는지

### 6-5. 경로 및 설정 파일 검증

Windows에서는 특히 실행 파일 경로와 사용자 홈 경로를 재확인한다.

- `C:\Users\<user>\...` 형태 절대경로
- 공백 포함 경로 (예: `C:\Users\<user>\Desktop\oct test\`)
- `%USERPROFILE%` 기준 설정/캐시 파일 생성 여부
- PATH에 `oct.exe`가 없는 경우에도 `os.Executable()` fallback이 정상 동작하는지

권장 확인 예시:

```powershell
where.exe oct
.\oct.exe config list
```

### 6-6. 검증 결과 기록 형식

Windows 검증 결과는 아래 형식으로 남긴다.

- 성공 조건: 어떤 명령이 어떤 환경에서 통과했는지
- 실패 로그: 에러 메시지 원문
- 수정 필요: 경로 quoting, Task Scheduler 등록값, fallback build 등 후속 조치

## 주의사항

- `agent-update`는 시스템의 `brew`, `npm`, `pnpm`, `yarn` 등을 실제로 호출하므로 의도치 않게 패키지가 업데이트될 수 있다.
- `update` 커맨드는 Go 버전에서 `npm install -g one-click-tools`를 실행하여 자기 자신을 업데이트한다.
- 새로운 기능을 추가한 경우 `go test`를 통해 기존 기능에 영향이 없는지 반드시 확인한다.

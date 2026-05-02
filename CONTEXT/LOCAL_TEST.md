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

## 주의사항

- `agent-update`는 시스템의 `brew`, `npm`, `pnpm`, `yarn` 등을 실제로 호출하므로 의도치 않게 패키지가 업데이트될 수 있다.
- `update` 커맨드는 Go 버전에서 `npm install -g one-click-tools`를 실행하여 자기 자신을 업데이트한다.
- 새로운 기능을 추가한 경우 `go test`를 통해 기존 기능에 영향이 없는지 반드시 확인한다.

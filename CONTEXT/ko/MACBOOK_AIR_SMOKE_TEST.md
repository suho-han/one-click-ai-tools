# MacBook Air Smoke Test 가이드

주 작업 macOS 호스트(`100.73.225.85`)에서 `one-click-tools`를 빠르게 검증하는 최소 smoke test 절차입니다.

## 대상 환경

- host: `100.73.225.85`
- user: `suhohan`
- repo: `/Users/suhohan/Projects/one-click-tools`
- 용도: 변경 후 빠른 정상 동작 확인

## 목적

이 문서는 다음을 빠르게 확인하는 용도입니다.
- 현재 작업 트리가 기대한 `main`/HEAD인지
- Go CLI가 빌드되는지
- 기본 Go 테스트가 깨지지 않았는지
- menubar helper 관련 핵심 Swift build/test가 유지되는지

전체 검증이나 원격 운영 절차는 다음 문서를 우선 참고합니다.
- `CONTEXT/ko/LOCAL_TEST.md`
- `CONTEXT/ko/MENUBAR_HELPER_OPERATIONS.md`
- `PROJECT_CONTEXT/remote-macos-validation-status.md`

## 빠른 시작

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/hermes_kbo_live_ed25519 suhohan@100.73.225.85
export PATH=/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin:$PATH
cd /Users/suhohan/Projects/one-click-tools
```

## 권장 smoke test 순서

### 1. 작업 트리 확인

```bash
git rev-parse --short HEAD
git status --short
git branch -vv | sed -n '1,5p'
```

체크 포인트:
- 기대한 브랜치/커밋인지 확인
- 불필요한 로컬 변경이 없는지 확인

### 2. Go CLI 빌드

```bash
go build -o oct main.go
./oct --version
./oct help >/dev/null
```

체크 포인트:
- `go build` 성공
- 빌드된 `./oct` 실행 가능

### 3. Go 테스트

```bash
GOTOOLCHAIN=auto go test ./...
```

체크 포인트:
- 전체 Go 테스트 통과
- CLI 명령/usage/schedule 관련 회귀 없음

### 4. Swift helper build

```bash
./oct menubar build-helper
```

체크 포인트:
- `macos/OctMenubar/.build/debug/OctMenubarApp` 생성
- Xcode/Swift toolchain 문제 없음

### 5. menubar snapshot 테스트

```bash
swift test --package-path macos/OctMenubar --filter UsageSnapshotTests
```

체크 포인트:
- 최근 menubar usage snapshot 기대값 회귀 없음
- timezone/date formatting 관련 변경이 테스트를 깨지 않는지 확인

## 필요 시 추가 smoke test

변경 범위에 따라 아래를 추가합니다.

### helper 진단/설치 확인

```bash
./oct menubar doctor
./oct menubar install-helper
./oct menubar doctor
```

확인 포인트:
- `launch mode: swift-helper`
- installed helper path가 `~/.local/bin/OctMenubarApp` 또는 repo build artifact로 resolve되는지 확인

### standalone helper 탐지 확인

```bash
cp ./oct /tmp/oct-standalone
/tmp/oct-standalone menubar doctor
```

확인 포인트:
- repo 밖 binary도 installed helper를 정상 탐지

### usage JSON smoke

```bash
./oct usage --json | head
```

확인 포인트:
- non-TTY 경로에서 JSON 출력이 깨지지 않음

## 권장 종료 정리

검증 후 생성 산출물이 불필요하면 정리합니다.

```bash
rm -f ./oct
rm -rf macos/OctMenubar/.build
```

주의:
- `node_modules`, `dist` 정리는 실제 작업 중이면 필요할 수 있으므로 무조건 지우지 않습니다.

## 2026-06-15 기준 확인된 baseline

다음 조합은 실제로 통과 확인되었습니다.

```bash
go test ./...
go build ./...
swift build --package-path macos/OctMenubar
swift test --package-path macos/OctMenubar --filter UsageSnapshotTests
```

당시 관련 커밋:
- `4581aae` `test(menubar): fix timezone expectations in usage snapshot test`

## 실패 시 우선 점검

1. `xcode-select -p`
2. `swift --version`
3. `go version`
4. `./oct menubar doctor`
5. `git status --short`

특히 원격 SSH에서는 PATH 문제보다 Xcode 선택 상태, Swift toolchain 상태, 작업 트리 오염 여부가 먼저 원인인 경우가 많습니다.

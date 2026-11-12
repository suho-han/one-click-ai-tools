# Statusline Parity (2026-11-12)

Status: planned, not implemented. Target: next release batch (deferred release).

## 목적

`oct usage --format waybar|polybar|swiftbar`는 이미 값을 출력하지만, 사용자가
바에 붙이려면 문서를 보고 설정 파일을 직접 작성해야 한다. roadmap P2의
"Cross-platform statusline/menubar parity outputs"를 CLI 설치 표면으로 마무리한다.

핵심은 **복붙 가능한 설정 생성 + 플러그인 파일 설치**다. Linux 트레이 데스크톱 앱은
의존성·유지비가 커서 범위 밖이다.

## 설계

### 새 커맨드 그룹 `oct statusline`

부모 커맨드 `statusline`은 서브커맨드 없이 실행하면 현재 지원 대상과 사용 예를
출력한다(설정 미변경).

1. `oct statusline config <waybar|polybar|swiftbar|xbar>`
   - 표준출력으로 복붙 가능한 설정 조각만 출력(설명은 stderr).
   - `waybar`: `custom/oct` 모듈 JSON (`exec: oct usage --format waybar`,
     `interval: 60`, `return-type: json`, `escape: false`)
   - `polybar`: `[module/oct]` INI (`type = custom/script`, `interval = 60`,
     `exec = oct usage --format polybar`)
   - `swiftbar`/`xbar`: 플러그인 메타데이터 헤더(`<bitbar.title>`, `version`,
     `oct usage --format swiftbar | ...`)를 포함한 스크립트 본문
   - `--path` 플래그로 실행 파일 경로를 덮어쓸 수 있음(기본 `oct`, PATH 가정)

2. `oct statusline install <swiftbar|xbar> [--dir <path>]`
   - 플러그인 파일 `oct.5m.sh`(0755) 생성. SwiftBar 기본 경로
     `~/Library/Application Support/SwiftBar/Plugins`, xbar는 `~/.config/xbar/plugins`.
   - 이미 존재하면 덮어쓰지 않고 `--force` 요구.
   - 설치 후 경로와 "SwiftBar 새로고침" 안내를 출력.

### 공통

- 렌더링은 기존 `--format` 구현을 그대로 호출하는 얇은 래퍼(중복 로직 금지).
- Windows는 shell 플러그인 대상이 아니므로 install은 명확한 오류로 거부.
- 문서: `context/usage.md` 상태라인 절과 README 커맨드 표에 `statusline` 행 추가.

## 테스트

- `cmd/statusline_test.go`
  - 각 대상의 golden 출력(경로 치환 포함)
  - 알 수 없는 대상 → exit 2 + 사용법
  - install: 파일 생성, 0755, 기존 파일 존재 시 거부·`--force` 덮어쓰기
  - Windows에서 install 거부(`runtime.GOOS` 가드)

## 검증

- `GOTOOLCHAIN=auto go test ./...`
- `go build -o oct main.go` 후 `./oct statusline config waybar`, `./oct statusline install xbar --dir $(mktemp -d)` 실행 확인

## 범위 제외

- Linux 트레이/데스크톱 위젯, menubar Swift UI 변경, 원격 설정 동기화

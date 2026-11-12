# Codex 계정 추가 시 로그인 방식 선택 (browser / device auth)

## 목적

`oct config` 피커의 "Add codex account…" 흐름이 지금은 무조건 `codex login`(브라우저)만
실행한다. SSH/원격/브라우저 콜백이 안 되는 환경에서는 로그인이 불가능하므로,
사용자가 두 방식 중 하나를 고를 수 있게 한다.

- 방식 1: `codex login` (기본값, 지금과 동일)
- 방식 2: `codex login --device-auth` (원격/SSH용 코드 입력 방식)

## 변경 범위

### 1. `cmd/config.go`

- `runInteractiveLogin(home string)` → `runInteractiveLogin(home string, deviceAuth bool)`.
  - `deviceAuth == false` → `exec.Command(bin, "login")`
  - `deviceAuth == true` → `exec.Command(bin, "login", "--device-auth")`
  - 환경변수 `CODEX_HOME=<home>`, stdin/stdout/stderr 연결은 그대로 유지.
- 새 헬퍼 `promptCodexLoginMethod() (deviceAuth bool, err error)`
  - 기존 `configPromptReader` / `readPromptLine()`를 그대로 사용한다.
  - 출력(메뉴):
    ```
    Codex login method:
      1) browser login (default) — opens a browser on this machine
      2) device auth — 'codex login --device-auth' (enter a code; use for SSH/remote)
    Choose [1/2] (default 1):
    ```
  - 입력이 비어 있거나(EOF 포함) 알 수 없는 값이면 기본값 1(browser)로 진행한다.
  - 새 bufio.Reader를 만들지 말 것 (같은 실행 안에서 뒤따르는 이름 입력 프롬프트가
    입력을 잃는다).
- `runAddCodexAccountFlow`
  - 로그인 직전에 `--device-auth` 플래그가 강제되지 않았다면 `promptCodexLoginMethod()`를 호출한다.
  - 선택 결과를 `runInteractiveLogin(tmpHome, deviceAuth)`로 넘긴다.
  - 실행 안내 문구를 선택에 맞춰 바꾼다:
    - browser: `Running 'codex login' — complete it in the browser...`
    - device: `Running 'codex login --device-auth' — enter the code it prints on another device...`
  - 실패 시 안내 문구에 선택한 방식을 반영한다.
- `runReconnectCodexAccountFlow`
  - 같은 플래그를 존중하되, **프롬프트는 추가하지 않는다**(기본 browser 유지).
    → `runInteractiveLogin(home, configDeviceAuth)`.
- 새 패키지 레벨 플래그 변수 `configDeviceAuth bool` + `configCmd`에
  `--device-auth` 플래그 등록(`Flags().BoolVar`). 플래그가 true면 프롬프트를 건너뛴다.
  - `oct config --device-auth` → 피커에서 Add codex account 선택 시 device auth로 로그인.
- `configAccountAddCmd`(등록만 수행, 로그인 없음)는 건드리지 않는다.

### 2. 테스트 `cmd/config_account_test.go`

- `runInteractiveLogin` 스텁 시그니처를 `func(home string, deviceAuth bool) error`로 수정.
- `stubLogin`은 마지막 `deviceAuth` 값도 캡처해 검증할 수 있게 확장한다.
- 기존 테스트 보정: 로그인 방식 프롬프트가 먼저 한 줄을 소비하므로,
  stdin을 주입하는 테스트(`TestRunAddCodexAccountFlow_EmailFallbackPrompts`)는
  `"1\nmyname\n"` 형태로 바꾼다.
- 신규 테스트:
  - `TestRunAddCodexAccountFlow_DeviceAuthChoice`: stdin `"2\n"` → `deviceAuth == true`.
  - `TestRunAddCodexAccountFlow_DefaultBrowserLogin`: stdin 없음(EOF) → `deviceAuth == false`.
  - `TestRunAddCodexAccountFlow_DeviceAuthFlagSkipsPrompt`: `configDeviceAuth = true`,
    stdin에 아무것도 없음 → `deviceAuth == true`이며 프롬프트 문구가 출력되지 않음.
    (테스트 종료 시 `configDeviceAuth` 원복)
- 기존 "No stdin: the flow must not ask anything" 주석은 새 동작에 맞게 갱신.

## 범위 제외

- `oct config account add` 서브커맨드 동작 변경 없음.
- device auth 진행 중 코드 표시 UI 개별 구현 없음 (codex CLI 출력을 그대로 통과).
- kimi/qwen/grok 계정 흐름 변경 없음.
- menubar/statusline 표시 변경 없음.
- 릴리스/버전 태그/커밋/푸시 없음.

## 검증

```bash
gofmt -l cmd internal
go build -o /tmp/oct main.go
GOTOOLCHAIN=auto go test ./...
```

수동 확인: `go run main.go config --help`에 `--device-auth`가 보이는지,
`printf '2\n' | go run main.go config` 경로에서 device auth 안내 문구가 나오는지
(codex 미설치 환경이면 로그인 spawn 실패 메시지로 확인).

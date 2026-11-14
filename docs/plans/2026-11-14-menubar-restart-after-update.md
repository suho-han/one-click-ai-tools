# oct menubar restart + 자동 재시작 (oct update 이후)

## 목적

`oct update`가 새 바이너리와 새 Swift menubar helper(OctMenubarApp)를 설치해도 이미
실행 중인 menubar 프로세스는 옛 inode/옛 oct 경로를 계속 사용한다. 업데이트 설치가
끝나면 실행 중인 menubar를 자동으로 재시작해서 새 빌드를 즉시 반영한다. 동시에
수동 재시작 수단으로 `oct menubar restart` 서브커맨드를 추가한다.

## CLI 표면

1. `oct menubar restart`
   - 현재 track(stable/beta)의 menubar 인스턴스를 모두 stop → 새 인스턴스 start.
   - 시작 경로 우선순위:
     a. 해당 track의 launchd daemon plist가 설치되어 있으면 `enableMenubarDaemon`을
        다시 실행한다. plist에 박제된 `OCT_MENUBAR_OCT_PATH`가 갱신되고
        (brew 업그레이드처럼 oct 경로가 바뀐 경우 필수), launchd `load`가
        RunAtLoad로 helper를 즉시 띄운다.
     b. plist가 없으면 기존 `startMenubarDetachedCommand()` 경로로 띄운다
        (Swift helper 있으면 helper, 없으면 legacy fallback + 기존 경고).
   - 성공 시 stdout: `menubar restarted`(pid 정보는 알 수 있으면 함께, 없으면 생략).
   - macOS가 아니거나 cgo 없는 darwin 빌드에서는 기존 menubar 커맨드들과 동일한
     수준의 에러 메시지를 반환한다.

2. `oct update`에 `--no-restart` 플래그 추가
   - 기본값 false(자동 재시작 수행). true면 업데이트 후 재시작을 생략한다.
   - `--check` 경로와 "이미 최신" 경로에서는 재시작하지 않는다.

## 자동 재시작 조건 (update 훅)

업데이트 설치가 **성공한 뒤** 아래를 만족할 때만 재시작한다.

- darwin && cgo 빌드.
- 해당 track의 menubar 인스턴스가 실행 중이거나, 해당 track의 daemon plist가
  설치되어 있음. 둘 다 아니면 아무 것도 하지 않는다(사용자가 menubar를 쓰지 않는
  경우 새로 띄우지 않는다).
- `--no-restart`가 아니고 `OCT_UPDATE_NO_MENUBAR_RESTART=1`이 아니며,
  `OCT_TELEMETRY_SOURCE=menubar`가 아닐 때. 마지막 조건은 menubar helper가 직접
  `oct update --yes`를 실행하는 경우로, helper가 이미 자체 Restart/Later 다이얼로그를
  띄우기 때문에 CLI가 그 아래에서 재시작하면 업데이트 결과를 기다리는 앱을 죽이게 된다.

적용 지점 두 곳:
- tarball 경로: `installReleaseAsset` 성공 직후 (`oct updated successfully ...` 출력 뒤).
- Homebrew 경로: `brew upgrade one-click-tools` 성공 직후.

실패 처리는 best-effort: 재시작 오류는 stderr `warning: menubar restart failed: ...`
로만 알리고 update 자체의 종료 코드는 성공(0)으로 유지한다. 재시작에 성공하면
stdout에 `menubar restarted to use the new build` 한 줄을 남긴다.

## 구현 위치

- `cmd/menubar.go`: `menubarRestartCmd` cobra 커맨드 등록(`stop` 옆), `menubarCmd.AddCommand`.
  실제 동작은 platform 파일의 `restartMenubarCommand(out io.Writer) error` 호출.
- `cmd/menubar_restart_darwin.go` (`//go:build darwin && cgo`): 실제 restart 로직.
  기존 심볼 재사용: `stopMenubarInstances`, `waitForMenubarExit`,
  `menubarDaemonLabel(currentMenubarTrack())`, `launchItemStatusCmd`,
  `enableMenubarDaemon`, `startMenubarDetachedCommand`.
- `cmd/menubar_restart_nocgo.go` (`//go:build darwin && !cgo`) 및
  `cmd/menubar_restart_stub.go` (`//go:build !darwin`): 기존 nocgo/stub 문구와 동일한 톤의 에러 반환.
- `cmd/update.go`: `--no-restart` 플래그, 두 성공 지점에서 훅 호출.
  훅은 platform별 `restartMenubarAfterUpdate(out io.Writer) (bool, error)`로 분리하고
  비-darwin은 `(false, nil)` no-op. 테스트가 스텁할 수 있도록 seam 변수
  (`restartMenubarAfterUpdateCmd`)로 감싼다.

## 제약

- 기존 `oct menubar`, `stop`, `daemon` 커맨드의 동작/출력 변경 금지.
- 새로운 서드파티 의존성 추가 금지.
- track 분리 유지: stable oct는 beta menubar를 건드리지 않고 그 반대도 동일.
- 재시작은 non-interactive이며 프롬프트를 띄우지 않는다(자동화에서 호출 가능해야 함).
- 로직은 `cmd/menubar_restart.go`(빌드 태그 없음)에 두고, platform 파일에는
  `menubarDaemonInstalled() bool`만 추가한다. 이렇게 하면 Linux `go build`/`go test`가
  실제 재시작 로직까지 타입체크한다.
- `OCT_MENUBAR_CHILD=1`(menubar가 띄운 child)에서도 동작해야 한다: 새 인스턴스는
  detached로 spawn되므로 caller가 곧 종료돼도 살아남는다. 별도 예외 처리 불필요.

## 테스트

- `cmd/menubar_restart_test.go`: `menubar restart`가 menubar 서브커맨드로 등록되어
  있고, `oct update`의 `--no-restart` 플래그가 존재하며 기본값이 false임을 확인.
- `cmd/update_test.go`: 업데이트 성공 경로에서 재시작 훅이 호출되고,
  `--no-restart`에서는 호출되지 않음을 seam stub으로 검증(훅이 실패해도 update가
  에러를 반환하지 않음도 포함).
- 훅 내부의 darwin 전용 분기는 실기기 검증 대상으로 남기고, 로컬에서는 빌드/테스트로만 확인.

## 검증 명령

```
gofmt -l cmd internal
go build -o oct main.go
GOTOOLCHAIN=auto go test ./...
```

macOS 실기기 검증(사용자 환경): `oct update --yes` 실행 후 menubar가 새 버전으로
재시작되는지, `oct menubar restart`가 단독으로 동작하는지 확인.

## 범위 제외

- menubar UI/설정에 재시작 버튼 추가.
- launchd plist 포맷 변경, helper 빌드 방식 변경.
- Windows/Linux에서 menubar 기능 추가.

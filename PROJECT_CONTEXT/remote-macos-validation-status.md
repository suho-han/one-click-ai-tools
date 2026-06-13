# Remote macOS Validation Status

## Summary

이 문서는 원격 macOS 호스트(`suhohan@100.114.89.25`)에서 `one-click-tools`의 최근 운영/검증 결과를 정리한다.

핵심 상태:
- 원격 글로벌 `oct` 설치본을 `0.4.12`에서 `0.4.14`로 갱신했다.
- 원격 **TTY SSH 세션**에서 `oct usage` interactive 출력이 정상 동작함을 확인했다.
- 원격 **non-interactive SSH** 환경은 기본 PATH에 `oct/node/npm`이 없지만, PATH bootstrap 후 `usage --json`, `agent-update`, `session-refresh`, `schedule` 검증이 모두 통과했다.
- 이번 작업에서 **저장소 소스 파일 수정은 없었고**, 원격 설치본/운영 상태 검증과 글로벌 설치 갱신만 수행했다.

## Environment confirmed

- host: `100.114.89.25`
- user: `suhohan`
- OS: `Darwin arm64`
- 원격 기본 non-interactive PATH:
  - `/Users/suhohan/flutter/bin:/usr/bin:/bin:/usr/sbin:/sbin`
- 원격 로그인/검증 시 추가로 사용한 PATH bootstrap:
  - `/opt/homebrew/bin:$PATH`

## Remote installation state

초기 상태:
- `oct --version` → `0.4.12`
- `command -v oct` → `/Users/suhohan/.nvm/versions/node/v24.15.0/bin/oct`
- 위 경로는 wrapper symlink였고, 실제 글로벌 install 구조를 추가 확인했다.

갱신 후 상태:
- `oct --version` → `0.4.14`
- `command -v oct` → `/opt/homebrew/bin/oct`
- symlink target:
  - `../lib/node_modules/one-click-tools/scripts/oct-wrapper.js`
- actual binary:
  - `/opt/homebrew/lib/node_modules/one-click-tools/bin/oct`
- actual binary type:
  - `Mach-O 64-bit executable arm64`

## Packaging / install actions performed

로컬에서 수행:
- `GOTOOLCHAIN=auto go build -o oct main.go`
- `./oct --version` → `0.4.14`
- `npm pack` → `one-click-tools-0.4.14.tgz`

원격에 수행:
- tarball 업로드:
  - `/tmp/one-click-tools-0.4.14.tgz`
- 글로벌 설치 갱신:
  - `OCT_INSTALL_ENABLE_SESSION_REFRESH=no npm install -g /tmp/one-click-tools-0.4.14.tgz`

비고:
- install 시 session-refresh prompt는 비활성화 환경변수로 막고 진행했다.
- 이번 검증은 published release가 아니라 **로컬 tarball 기반 원격 글로벌 설치 갱신**이다.

## TTY validation results

원격 SSH TTY 세션에서 확인:
- `oct --version` 정상
- `oct usage` 실행 시 non-TTY fallback이 아니라 spinner + table 기반 interactive 경로 동작

확인된 의미:
- 원격 TTY 세션에서는 `cmd/usage.go`의 TTY 경로가 정상적으로 선택된다.
- 운영자가 터미널에서 직접 접속해 쓸 때 interactive UX가 유지된다.

## Non-interactive SSH validation results

### 1. Baseline failure condition

plain non-interactive SSH에서 기본 PATH는 다음과 같았다.
- `__PATH__=/Users/suhohan/flutter/bin:/usr/bin:/bin:/usr/sbin:/sbin`

이 상태에서는 다음이 모두 미검출이었다.
- `oct`
- `node`
- `npm`

즉, remote automation / plain SSH command path 에서는 shell bootstrap 없이는 CLI 실행을 기대하면 안 된다.

### 2. Bootstrap success condition

다음 bootstrap을 추가하면 비대화형 SSH 경로가 정상 동작했다.
- `export PATH=/opt/homebrew/bin:$PATH`

검증 통과 명령:
- `oct usage --json`
- `oct agent-update`
- `oct session-refresh --json`
- `oct schedule ...`

## usage --json result (non-interactive)

검증 시점 summary:
- `total=6`
- `ok=2`
- `warn=4`
- `error=0`

주요 provider 상태:
- `claude-code` → ok
- `codex` → ok
- `antigravity` → ok
- `copilot` → ok
- `cursor` → warn
- `opencode` → warn

의미:
- non-interactive SSH + PATH bootstrap 조합에서 JSON mode 사용은 안정적이다.

## agent-update validation (non-interactive)

원격 plain SSH + PATH bootstrap 경로에서 `oct agent-update`를 실행해 완료까지 확인했다.

확인된 manager matrix:
- Antigravity CLI → `antigravity-installer`
- Cursor CLI → `cursor-agent`
- OpenAI Codex → `brew`
- Claude Code → `brew`
- GitHub Copilot → `brew`
- OpenCode → `npm`

최종 결과:
- `All tools updated successfully!`

의미:
- 이 원격 macOS host에서는 non-interactive shell에서도 PATH만 bootstrap 되면 manager detection / update path 가 정상적으로 끝난다.
- Copilot brew target 관련 이전 known issue는 현재 검증 결과상 정상 처리된다.

## session-refresh validation

실행:
- `oct session-refresh --json`

확인된 refresh 결과:
- `claude` → `skipped` / auth not logged in
- `codex` → `ok` / logged in using ChatGPT
- `agy` → `ok` / local session artifacts detected
- `copilot` → `skipped` / partial auth only
- `cursor-agent` → `skipped` / local auth.json 없음
- `opencode` → `ok` / credential inventory detected

추가 확인:
- `usage --json` 실행 결과를 `session-refresh` 전후로 비교했을 때 summary와 provider 상태가 실질적으로 동일했다.

해석:
- 현재 `session-refresh`는 usage/quota 값을 적극 갱신하는 명령이 아니라,
  - provider별 token-free / local-artifact 기반 probe를 수행하고,
  - 이후 usage collection을 다시 실행하는 구조다.
- 즉, 현재 의미는 **session/auth health probe + usage recollect**에 가깝다.

## schedule validation

검증 대상:
- `session-refresh` task

수행 순서:
1. `oct schedule --task session-refresh`
2. `oct schedule enable --task session-refresh --interval daily --hour 9`
3. `oct schedule --task session-refresh`
4. plist 생성/내용 확인
5. `launchctl list | grep com.oct.session-refresh`
6. `oct schedule disable --task session-refresh`
7. `oct schedule --task session-refresh`
8. plist 제거 확인

결과:
- before → `disabled`
- enable 후 → `enabled`
- disable 후 → `disabled`

생성된 plist:
- path:
  - `/Users/suhohan/Library/LaunchAgents/com.oct.session-refresh.plist`
- ProgramArguments:
  - `/opt/homebrew/lib/node_modules/one-click-tools/bin/oct`
  - `session-refresh`
- log path:
  - `/Users/suhohan/.oct/logs/session-refresh.log`
- `launchctl list`에 `com.oct.session-refresh` 로드 확인
- disable 후 plist 제거 확인 (`MISSING`)

중요한 확인 포인트:
- 등록 엔트리는 PATH 상 wrapper가 아니라 **실제 current installed binary path**를 가리켰다.
- macOS schedule lane의 binary-path correctness 요구사항을 충족한다.

## Current remote config scope

원격 `~/.oct/config.yaml`에서 확인된 현재 enabled tool scope:
- `claude`
- `codex`
- `gemini`
- `copilot`
- `cursor-agent`
- `opencode`

즉, 검증 당시 `agent-update`는 partial selection이 아니라 6개 전체 tool scope를 기준으로 수행되었다.

## Important operational notes

### 1. PATH caveat remains

현재도 plain non-interactive SSH는 기본적으로 `oct/node/npm`을 찾지 못한다.

운영 자동화 시 권장 패턴:
- `export PATH=/opt/homebrew/bin:$PATH; <oct command>`
- 또는 login shell/bootstrap을 먼저 보장한 뒤 실행

### 2. No repository source diff in this session

이번 세션은 다음 범위에 한정되었다.
- 원격 글로벌 설치 갱신
- 원격 운영 검증
- TTY / non-interactive / session-refresh / schedule 실동작 검증

포함되지 않은 것:
- Go/Node 소스 코드 수정
- 테스트 추가
- 문서 외 코드 커밋

### 3. Secret hygiene follow-up

원격 config 확인 과정에서 민감한 credential field가 노출될 수 있으므로,
- 채널 로그 보존 범위가 넓거나
- 외부 공유 가능성이 있다면
해당 token rotation을 별도 후속 조치로 고려하는 것이 안전하다.

## Suggested follow-ups

우선순위가 높은 후속 작업:
1. non-interactive SSH에서도 항상 동작하도록 remote shell/profile/bootstrap strategy 정리
2. 이 macOS host 기준 운영 체크리스트를 `CONTEXT/` 또는 docs로 승격
3. 필요 시 `agent-update`, `session-refresh`, `schedule`에 대한 remote macOS regression note 추가
4. published npm release 기준으로도 동일 설치/검증 절차를 한 번 더 확인

## Menubar helper validation (2026-06-14)

원격 macOS fresh workspace에서 현재 작업 트리를 별도 압축 업로드해 다음을 실검증했다.

실행 순서:
1. `/tmp/oct-remote-validate` 에 현재 작업 트리 unpack
2. `go build -o oct .`
3. `./oct menubar doctor`
4. `./oct menubar build-helper`
5. `./oct menubar install-helper`
6. `./oct menubar doctor`
7. standalone binary(`/tmp/oct-standalone`) 기준 `menubar doctor`

검증 결과:
- build 전 `menubar doctor`
  - `launch mode: legacy-fallback`
  - helper 미발견
  - helper project는 `/tmp/oct-remote-validate/macos/OctMenubar` 로 정상 탐지
- `menubar build-helper`
  - Swift helper build 성공
  - 생성 binary: `/tmp/oct-remote-validate/macos/OctMenubar/.build/debug/OctMenubarApp`
- `menubar install-helper`
  - 설치 성공
  - install path: `/Users/suhohan/.local/bin/OctMenubarApp`
  - file type: `Mach-O 64-bit executable arm64`
- build 후 same-worktree `menubar doctor`
  - `launch mode: swift-helper`
  - helper path가 build artifact로 resolve됨
- standalone binary 기준 `menubar doctor`
  - `launch mode: swift-helper`
  - helper path가 `/Users/suhohan/.local/bin/OctMenubarApp` 로 resolve됨

의미:
- macOS에서 `menubar build-helper` / `install-helper` 전체 flow는 실제로 성공한다.
- repo 밖 standalone `oct` binary도 install된 helper(`~/.local/bin/OctMenubarApp`)를 정상 탐지한다.
- 즉 helper resolution 우선순위가 dev-worktree와 installed-helper 두 경우 모두 기대대로 동작한다.

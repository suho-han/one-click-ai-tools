# Release Checklist (npm / pnpm)

## Goal
태그 릴리즈 시 `oct` 바이너리 버전, `package.json` 버전, Git 태그가 항상 일치하도록 보장하고 npm/pnpm publish 실패 가능성을 사전 차단합니다.

## Preflight (로컬)
1. 워킹트리 clean 확인
   - `git status --short`
2. 버전 정합성 + 패키징 점검
   - `bash scripts/verify-release-integrity.sh`
3. 빌드/테스트
   - `go test ./...`
   - `go build ./...`

## Release (로컬 자동화)
- npm publish
  - `bash scripts/release-package.sh npm`
  - 또는 `npm run release:npm`
- pnpm publish
  - `bash scripts/release-package.sh pnpm`
  - 또는 `npm run release:pnpm`

공통 동작:
- `standard-version`으로 버전/태그 생성
- `verify-release-integrity.sh` 실행
- `go test ./...`
- `go build ./...`
- publish dry-run 실행
- `git push --follow-tags`
- 선택한 manager로 publish 실행

호환성 wrapper:
- `bash scripts/publish.sh` 는 계속 npm release wrapper로 동작

## CI Release Guard (`.github/workflows/release.yml`)
CI release job는 계속 `npm publish`를 canonical path로 사용합니다.
`npm-publish` job에서 아래 순서로 검증합니다.
1. `go build -o oct main.go`
2. `bash scripts/verify-release-integrity.sh` (`RELEASE_TAG` 주입)
3. `npm publish --dry-run --access public`
4. `npm publish --access public`

정리 원칙:
- 로컬 배포 루틴은 `npm`/`pnpm` 둘 다 지원
- GitHub release CI의 registry publish는 `npm`을 기준으로 유지

## Manager stability guard
추가 패키지 매니저 지원 또는 manager detection 변경 시 아래를 같이 확인합니다.

### Built-in manager support matrix

| Tool | expected install manager | notes |
| --- | --- | --- |
| Claude Code | `npm` fallback or detected `brew`/`pnpm`/`yarn` | provenance-first detection 우선 |
| Cursor CLI | `cursor-agent` | 공식 installer 고정 |
| OpenCode | `npm` fallback or detected `brew`/`pnpm`/`yarn` | provenance-first detection 우선 |
| OpenAI Codex | `npm` fallback or detected `brew`/`pnpm`/`yarn` | provenance-first detection 우선 |
| Antigravity CLI (`agy`) | `antigravity-installer` | `curl -fsSL https://antigravity.google/cli/install.sh | bash` |
| GitHub Copilot | `npm` fallback | npm local-prefix fallback 포함 |

1. provenance-first detection 회귀 테스트
   - `GOTOOLCHAIN=auto go test ./internal/update -run 'DetectManager|PreferredBinaries|ResolveManagerForInstall|SupportMatrix' -v`
2. 전체 update 패키지 테스트
   - `GOTOOLCHAIN=auto go test ./internal/update -v`
3. binary path correctness 검토
   - 실제 실행 binary path가 manager prefix(`brew --prefix`, `pnpm bin -g`, `npm prefix -g`, `go env GOPATH`, `python3 -m site --user-base`)와 일치하는지 확인
4. dedicated installer 검토
   - Cursor / Antigravity처럼 package-manager가 아닌 공식 installer flow는 `DetectManager()`와 `ResolveManagerForInstall()` 둘 다 전용 manager를 유지해야 함
5. ambiguous fallback 검토
   - `DetectManager()`는 `Unknown`을 반환할 수 있어야 함
   - install/update 경로만 `ResolveManagerForInstall()`로 default manager fallback 허용

## Failure / Rollback Guide
- 버전 불일치 실패 시
  - `cmd/root.go`의 `Version`과 `package.json`의 `version`을 동일하게 수정
  - 재커밋 후 태그 재생성
- npm dry-run 실패 시
  - `npm pack --dry-run` 로컬 재현 후 포함 파일/스크립트 확인
- 잘못된 태그 푸시 시
  - 로컬/원격 태그 삭제 후 올바른 버전으로 재태깅

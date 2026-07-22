# Release Checklist (GitHub Releases)

## Goal
태그 릴리즈 시 `oct` 바이너리 버전, Git 태그, GitHub Release assets/checksums가 일치하도록 보장합니다.

## Preflight (로컬)
1. 워킹트리 clean 확인
   - `git status --short`
2. 버전 확인
   - `go run main.go --version`
3. 버전 정합성 + 빌드 점검
   - `bash scripts/verify-release-integrity.sh`
4. 테스트
   - `GOTOOLCHAIN=auto go test ./...`
   - `GOTOOLCHAIN=auto go build ./...`

## Release (로컬 자동화)
- GitHub Release
  - `bash scripts/release-package.sh vX.Y.Z`

동작:
- `cmd/root.go` 버전 갱신
- release commit/tag 생성
- `verify-release-integrity.sh` 실행
- `go test ./...`
- `git push --follow-tags`
- GitHub Actions `goreleaser` workflow 완료 대기
- GitHub Release asset 목록 확인

호환성 wrapper:
- `bash scripts/publish.sh vX.Y.Z` 는 GitHub Release wrapper로 동작합니다.

## CI Release Guard (`.github/workflows/release.yml`)
CI release job는 GitHub Releases를 canonical distribution path로 사용합니다.
트리거:
- `push` on `v*` tags
- `workflow_dispatch` with `release_mode` (`snapshot` / `release`) and `git_ref=vX.Y.Z`

`verify-release-assets` job에서 아래 순서로 검증합니다.
1. `bash scripts/verify-release-integrity.sh` (`RELEASE_TAG=${EFFECTIVE_RELEASE_TAG}`)
2. release asset presence check (`gh release view ... --json assets`)
3. `checksums.txt` 존재 확인

수동 재실행 경로:
- GitHub Actions → `goreleaser` → `Run workflow`
- `release_mode=release`
- `git_ref=vX.Y.Z` 지정

정리 원칙:
- package registry publish는 사용하지 않습니다.
- 사용자는 `scripts/install.sh` 또는 `oct update`로 GitHub Release 바이너리를 설치/갱신합니다.

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
- GitHub Release asset 누락 시
  - `.github/workflows/release.yml`의 `goreleaser`, `darwin-assets`, `verify-release-assets` job 순서 확인
  - `gh release view vX.Y.Z --json assets`로 asset 확인
- 버전 불일치 실패 시
  - `cmd/root.go`의 `Version`과 release tag를 동일하게 수정
  - 재커밋 후 태그 재생성
- checksum 누락 시
  - `darwin-assets` job이 macOS tarball checksum을 `checksums.txt`에 병합했는지 확인
- 잘못된 태그 푸시 시
  - 로컬/원격 태그 삭제 후 올바른 버전으로 재태깅

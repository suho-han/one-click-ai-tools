# Release Checklist (npm)

## Goal
태그 릴리즈 시 `oct` 바이너리 버전, `package.json` 버전, Git 태그가 항상 일치하도록 보장하고 npm publish 실패 가능성을 사전 차단합니다.

## Preflight (로컬)
1. 워킹트리 clean 확인
   - `git status --short`
2. 버전 정합성 + 패키징 점검
   - `bash scripts/verify-release-integrity.sh`
3. 빌드/테스트
   - `go test ./...`
   - `go build ./...`

## Release (로컬 자동화)
- `bash scripts/publish.sh`
  - `standard-version`으로 버전/태그 생성
  - `git push --follow-tags`
  - `verify-release-integrity.sh` 실행
  - `npm publish` 실행

## CI Release Guard (`.github/workflows/release.yml`)
`npm-publish` job에서 아래 순서로 검증합니다.
1. `go build -o oct main.go`
2. `bash scripts/verify-release-integrity.sh` (`RELEASE_TAG` 주입)
3. `npm publish --dry-run --access public`
4. `npm publish --access public`

## Failure / Rollback Guide
- 버전 불일치 실패 시
  - `cmd/root.go`의 `Version`과 `package.json`의 `version`을 동일하게 수정
  - 재커밋 후 태그 재생성
- npm dry-run 실패 시
  - `npm pack --dry-run` 로컬 재현 후 포함 파일/스크립트 확인
- 잘못된 태그 푸시 시
  - 로컬/원격 태그 삭제 후 올바른 버전으로 재태깅

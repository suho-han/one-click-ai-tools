# Menubar Helper 운영 가이드

macOS menubar helper(`OctMenubarApp`)의 빌드, 설치, 진단, 원격 검증 절차를 정리한 운영 문서입니다.

## 대상 범위

이 문서는 다음 상황을 다룹니다.
- macOS 개발기에서 Swift helper를 직접 빌드/설치할 때
- repo 내부 개발 빌드와 standalone `oct` binary의 helper 탐지 우선순위를 확인할 때
- 원격 macOS 호스트에서 SSH로 helper flow를 검증할 때

## 핵심 명령

```bash
oct menubar doctor
oct menubar build-helper
oct menubar install-helper
```

각 명령의 의미:
- `doctor`: 현재 launch mode, helper 탐지 경로, helper project 위치를 진단
- `build-helper`: `macos/OctMenubar` Swift helper를 로컬에서 빌드
- `install-helper`: 빌드된 helper를 사용자 실행 경로에 설치

## 기대 동작

### 1. helper 미설치 상태

`oct menubar doctor` 예시:
- `launch mode: legacy-fallback`
- helper 미발견
- helper project는 `macos/OctMenubar`로 탐지됨

의미:
- Swift helper가 아직 build/install 되지 않았고 기존 fallback 실행 경로를 사용할 상태

### 2. repo 내부에서 build 직후

```bash
oct menubar build-helper
oct menubar doctor
```

기대 결과:
- helper build 성공
- build artifact 예: `macos/OctMenubar/.build/debug/OctMenubarApp`
- `doctor`에서 `launch mode: swift-helper`
- helper path가 worktree build artifact로 resolve

의미:
- 개발 중에는 install 전이라도 repo 내부 build artifact를 우선 사용 가능해야 함

### 3. install 후

```bash
oct menubar install-helper
oct menubar doctor
```

기대 결과:
- install path: `~/.local/bin/OctMenubarApp`
- binary type: `Mach-O 64-bit executable arm64` (Apple Silicon 기준)
- `doctor`에서 `launch mode: swift-helper`

의미:
- 개발 worktree 밖에서도 설치된 helper를 통해 menubar 실행 가능해야 함

## standalone binary 검증

repo 밖에 복사한 binary에서도 installed helper를 탐지하는지 확인합니다.

```bash
cp ./oct /tmp/oct-standalone
/tmp/oct-standalone menubar doctor
```

기대 결과:
- `launch mode: swift-helper`
- helper path가 `~/.local/bin/OctMenubarApp` 로 resolve

의미:
- standalone 배포 binary도 installed helper를 정상 사용함

## 원격 macOS 검증 절차

원격 호스트 예:
- host: `100.114.89.25`
- user: `suhohan`

fresh workspace 예:
- `/tmp/oct-remote-validate`

권장 순서:
1. 현재 작업 트리를 원격 임시 디렉터리에 업로드/압축 해제
2. `go build -o oct .`
3. `./oct menubar doctor`
4. `./oct menubar build-helper`
5. `./oct menubar install-helper`
6. `./oct menubar doctor`
7. `/tmp/oct-standalone menubar doctor`

## 2026-06-14 실검증 결과

원격 macOS에서 실제로 확인된 사항:
- `build-helper` 성공
- `install-helper` 성공
- installed helper path: `/Users/suhohan/.local/bin/OctMenubarApp`
- installed helper type: `Mach-O 64-bit executable arm64`
- same-worktree `doctor`는 build artifact를 helper path로 선택
- standalone `doctor`는 installed helper를 helper path로 선택

즉 다음 두 경우가 모두 확인됨:
- dev worktree 우선 탐지
- installed helper fallback 탐지

## 운영 주의사항

### non-interactive SSH PATH

원격 macOS의 plain non-interactive SSH는 기본 PATH에 Homebrew bin이 빠질 수 있습니다.

권장 bootstrap:

```bash
export PATH=/opt/homebrew/bin:$PATH
```

이 bootstrap 없이 원격 자동화에서 `oct`, `node`, `npm` 탐지가 실패할 수 있습니다.

### build prerequisites

`build-helper`는 macOS + Swift/Xcode toolchain이 있어야 합니다.
Linux 호스트에서는 helper build를 검증할 수 없습니다.

## 장애 시 점검 순서

1. `oct menubar doctor`
2. `macos/OctMenubar` 디렉터리 존재 확인
3. `oct menubar build-helper`
4. `~/.local/bin/OctMenubarApp` 존재 및 실행권한 확인
5. standalone binary 기준 `menubar doctor` 재확인

## 관련 문서

- `README.md`: 사용자-facing quick start / command overview
- `PROJECT_CONTEXT/remote-macos-validation-status.md`: 원격 검증 상세 로그와 상태 기록
- `CONTEXT/ko/LOCAL_TEST.md`: 일반 로컬 빌드/테스트 가이드

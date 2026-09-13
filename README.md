# one-click-ai-tools (oct)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

[English](README.en.md)

> **AI 코딩 CLI를 하나의 바이너리로 정리합니다 — 전부 업데이트, 전부 쿼터 감시, 유지보수 예약.**

oct는 흩어져 있는 AI 코딩 도구 관리를 하나로 모으는 단일 Go 바이너리 CLI입니다.

- **전부 업데이트** — Claude Code, Codex, Copilot, Cursor 등 10개 CLI를 설치 매니저 자동 감지(brew/npm/공식 인스톨러)로 한 번에 업데이트
- **전부 쿼터 감시** — 14개 프로바이더의 구독 쿼터를 하나의 테이블 / JSON / macOS 메뉴바로 표시
- **유지보수 예약** — 업데이트와 세션 점검을 launchd / cron / SchTasks에 예약하고, 임계값 도달 시 OS 알림

## 왜 oct인가

사용량 조회 도구는 업데이트를 못 하고, 업데이터는 사용량을 못 봅니다. oct는 이 둘을 한 바이너리로 묶은 것이 핵심입니다.

- **단일 Go 바이너리** — Node/Python 런타임 불필요, macOS / Linux / Windows
- **하이브리드 수집** — usage API가 있는 프로바이더는 API를 직접 조회해 남은 비율과 상태를 확인하고, API가 없는 도구(Qwen 등)는 로컬 사용 기록으로 보완
- **자격증명 재사용** — 각 CLI가 이미 저장한 OAuth 토큰·API 키를 그대로 사용, 별도 로그인 없음
- **매니저 자동 감지** — 도구마다 설치된 매니저를 찾아 맞는 업데이트 명령을 실행 (회귀 테스트로 고정)

## 빠른 시작

### 설치 (GitHub Releases)

```bash
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | sh
```

스크립트는 현재 OS/CPU에 맞는 바이너리를 내려받고, 릴리스 checksum 항목이 있으면 검증한 뒤 기본적으로 `~/.local/bin/oct`에 설치합니다. 터미널에서 실행하면 설치 직후 `oct config`가 자동으로 열립니다.

```bash
# 특정 버전 설치
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_VERSION=v0.1.5 sh

# 설치 경로 변경
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_DIR=/usr/local/bin sh

# 설치 후 config 단계 건너뛰기
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_RUN_CONFIG=0 sh
```

### 처음 5분

```bash
# 1. 사용할 도구/프로바이더 선택 (인터랙티브)
oct config

# 2. 설치된 AI CLI 전부 업데이트 — 실행 전 계획 확인 가능
oct agent-update --dry-run --explain
oct agent-update

# 3. 전체 쿼터 현황 확인
oct usage            # 사람용 테이블
oct usage --json     # 스크립트/파이프용
```

## 명령어 지도

| 단계 | 명령어 | 하는 일 |
| --- | --- | --- |
| 설정 | `oct config` | 도구·프로바이더 선택, 사용량 표시 모드 설정 (인터랙티브) |
| 업데이트 | `oct agent-update` | 설치된 AI CLI 전부 업데이트 (`--dry-run --explain` 사전 점검) |
| 감시 | `oct usage` | 전 프로바이더 쿼터 1회 조회 (`--json`, `--compact`, `--notify`) |
| 감시 | `oct monitor` | 상시 갱신 화면 (`--interval`, `--once`, 정렬·필터) |
| 감시 | `oct menubar` | macOS 메뉴바에 상시 표시 |
| 알림 | `oct alert` | 임계값 도달 시 OS 알림 (quiet hours, snooze 지원) |
| 예약 | `oct schedule` | agent-update / session-refresh를 OS 스케줄러에 등록 |
| 예약 | `oct session-refresh` | 프롬프트 없이 세션·인증 상태만 probe (`--dry-run`) |
| 진단 | `oct doctor` | shell PATH / bootstrap 진단 |
| 진단 | `oct update` | oct 자체 업데이트 |
| 개발 | `oct release-doctor` | 릴리스 전 점검 한 번에 보기 |

`oct --help`는 사용 빈도 기준으로 그룹핑해 보여줍니다 (Core / Configuration & Scheduling / Update & Maintenance).

### 1) 전부 업데이트 — `oct agent-update`

```bash
oct agent-update                      # 전체 업데이트 실행
oct agent-update --dry-run --explain  # 실행 없이 도구별 감지 매니저와 계획만 출력
```

도구마다 설치 경로와 매니저를 감지합니다. 지원 매니저: `brew`, `npm`, `pnpm`, `yarn`, `cargo`, `go-install`, `pip`, Cursor/Antigravity 공식 인스톨러 — 상세 매트릭스는 아래 [Manager Support Matrix](#manager-support-matrix).

### 2) 전부 쿼터 감시 — `oct usage` / `oct monitor` / `oct menubar` / `oct alert`

```bash
oct usage                        # 1회 조회
oct usage --compact              # C-45% X-25% 형태 요약
oct usage --json                 # JSON 출력
oct usage --notify               # 임계값 규칙에 따라 알림 발송

oct monitor --interval 10s       # 10초 갱신 상시 화면
oct monitor --once --sort-by used --desc --top 5 --compact

oct alert config show
oct alert config set enabled true
oct alert config set threshold_percent 85
oct alert config set quiet_hours 00:00-08:00
oct alert snooze set --duration 2h
```

### 3) 유지보수 예약 — `oct schedule` / `oct session-refresh`

```bash
oct schedule enable --task agent-update --interval daily --hour 9   # 매일 9시 전체 업데이트
oct schedule enable --task session-refresh --interval 6h            # 6시간마다 세션 점검
oct schedule --task agent-update                                    # 등록 상태 확인
oct session-refresh --dry-run                                       # 토큰 소모 없이 수동 점검
```

## 지원 AI 에이전트 (업데이트 대상)

- **Claude Code** (`@anthropic-ai/claude-code`)
- **Command Code** (`command-code`, binary: `commandcode` / `cmd`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (공식 인스톨러, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (공식 `agent` 설치 흐름, `cursor.com/install`)
- **OpenCode** (`opencode-ai`)
- **Kimi Code** (`@moonshot-ai/kimi-code`, binary: `kimi`)
- **Qwen Code** (`@qwen-code/qwen-code`, binary: `qwen`)
- **MiniMax** (`mmx-cli`, binary: `mmx`)

### 사용량 전용 프로바이더 (설치 대상 아님, opt-in)

설치/업데이트 대상 CLI는 아니지만 사용량 조회만 지원합니다. `agent_order` 또는 `enabled_tools`에 이름을 넣으면 표에 나타납니다 (예: `oct config set tools zai`).

- **Z.ai (GLM Coding Plan)** — `ZAI_API_KEY` / `ZHIPU_API_KEY`, 또는 opencode `zai-coding-plan` 로그인
- **DeepSeek** — `DEEPSEEK_API_KEY`
- **OpenRouter** — `OPENROUTER_API_KEY`
- **Grok (xAI SuperGrok)** — `grok login` 자격증명 또는 `GROK_OAUTH_TOKEN`

## 메뉴바 헬퍼 (macOS)

Swift menubar helper를 따로 빌드/설치할 수 있습니다.

```bash
oct menubar                # 메뉴바 앱 실행
oct menubar doctor         # helper 탐색/launch 상태 점검
oct menubar build-helper   # Swift helper build
oct menubar install-helper # ~/.local/bin/OctMenubarApp 로 설치
```

## Manager Support Matrix

| Manager | 감지 기준 | 설치 경로 | built-in 사용처 |
| --- | --- | --- | --- |
| `brew` | `brew --prefix` 하위 binary 또는 `brew list` | `brew upgrade <formula>` | Homebrew로 설치된 Claude/Cursor/OpenCode/Codex |
| `npm` | `npm prefix -g` 또는 `npm list -g` | `npm install -g <package>` | Claude/Command Code/OpenCode/Codex/Copilot 기본 fallback |
| `pnpm` | `pnpm bin -g` 또는 `pnpm list -g` | `pnpm add -g <package>` | provenance 기반 감지만 지원 |
| `yarn` | `yarn global bin` 또는 `yarn global list` | `yarn global add <package>` | provenance 기반 감지만 지원 |
| `cargo` | `cargo:` package prefix 또는 cargo bin path | `cargo install <crate> --locked` | explicit package override |
| `go-install` | `go:` package prefix 또는 `go env GOPATH` bin path | `go install <package>@latest` | explicit package override |
| `pip` | `pip:` package prefix 또는 `python3 -m site --user-base` bin path | `python3 -m pip install --upgrade <package>` | explicit package override |
| `cursor-agent` | tool identity (`cursor-agent` / `cursor` / `agent`) | `curl https://cursor.com/install -fsS \| bash` | Cursor CLI |
| `antigravity-installer` | tool identity (`agy` / `antigravity`, legacy `gemini*`) | `curl -fsSL https://antigravity.google/cli/install.sh \| bash` | Antigravity CLI |

이 built-in support matrix는 `internal/update/manager_test.go`에서 회귀 테스트로 고정합니다.

## Requirements

- **사용자**: macOS agent-update 지원에는 Homebrew (Linux/Windows는 CLI 기능만)
- **개발자 (소스 빌드/테스트)**: **Go >= 1.25**

## Release

- 기본 배포 채널: GitHub Releases + `scripts/install.sh`
- 로컬 릴리스 래퍼: `bash scripts/release-package.sh vX.Y.Z`

## License

MIT © Suho Han

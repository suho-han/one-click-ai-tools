# one-click-tools (oct)

[![npm version](https://img.shields.io/npm/v/one-click-tools.svg?style=flat-square)](https://www.npmjs.com/package/one-click-tools)
[![pnpm](https://img.shields.io/badge/maintained%20with-pnpm-cc00ff.svg?style=flat-square&logo=pnpm)](https://pnpm.io/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

[English](README.en.md)

**one-click-tools (oct)** 는 주요 AI 개발 도구를 한 번에 설치/업데이트할 수 있는 고성능 CLI입니다.

## 🚀 빠른 시작

### 설치

```bash
# npm 설치
npm install -g one-click-tools

# pnpm 설치
pnpm add -g one-click-tools
```

### 기본 사용

```bash
# 모든 AI 에이전트 업데이트 (Claude, Codex, Gemini, Copilot, Cursor, OpenCode)
oct agent-update

# AI 도구 사용량/쿼터 확인
oct usage

# 도움말
oct help
```

## ✅ 쉬운 사용법 (지금 바로 쓰는 명령어)

아래 4가지만 알면 대부분 바로 사용할 수 있습니다.

### 1) 에이전트 업데이트

```bash
# Claude/Codex/Gemini/Copilot/Cursor/OpenCode 업데이트
oct agent-update
```

### 2) 사용량 확인

```bash
# 현재 사용량 확인
oct usage
```

동작 기준:
- `enabled_tools`에 포함된 provider만 `usage`/`monitor`에서 조회합니다.
- 출력 순서는 `agent_order`를 따릅니다.

### 3) 상시 모니터링 화면

```bash
# 10초마다 갱신
oct monitor --interval 10s

# 1회만 확인
oct monitor --once

# 사용량 높은 순으로 상위 5개만 간단히 보기
oct monitor --once --sort-by used --desc --top 5 --compact
```

### 4) 사용량 알림 설정

```bash
# 알림 설정 보기
oct alert config show

# 알림 켜기/기본값 설정
oct alert config set enabled true
oct alert config set cooldown_minutes 120
oct alert config set threshold_percent 85
oct alert config set critical_percent 98
oct alert config set quiet_hours 00:00-08:00
oct alert config set timezone Asia/Seoul
```

설정값 설명:

- `enabled`: 사용량 알림 기능 on/off
- `cooldown_minutes`: 같은 provider/window 알림 재전송 최소 간격(분)
- `threshold_percent`: 기본 경고 임계치(%)
- `critical_percent`: CRITICAL 등급 임계치(%), quiet hours/snooze보다 우선
- `quiet_hours`: 일반 알림 무음 시간대 (`HH:MM-HH:MM`)
- `timezone`: quiet hours 계산 기준 타임존 (예: `Asia/Seoul`)

#### 세부 임계치(윈도우/프로바이더별)

```bash
# 전역 윈도우별
oct alert config set threshold.5h 90
oct alert config set threshold.7d 92

# provider별
oct alert config set provider.codex.5h 94
oct alert config set provider.codex.default 88
oct alert config set provider.cursor.5h 93
oct alert config set provider.opencode.default 87
```

#### 일시 정지(snooze)

```bash
# 전체 알림 2시간 일시정지
oct alert snooze set --duration 2h

# 특정 provider/window만 일시정지
oct alert snooze set --duration 1h --provider codex --window 5h
oct alert snooze set --duration 1h --provider cursor --window 5h
oct alert snooze set --duration 1h --provider opencode --window 7d

# 확인/해제
oct alert snooze show
oct alert snooze clear --provider codex --window 5h
```

## 🛠 지원 에이전트

- **Claude Code** (`@anthropic-ai/claude-code`)
- **OpenAI Codex** (`@openai/codex`)
- **Gemini CLI** (`@google/gemini-cli`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor** (`cursor-agent`)
- **OpenCode** (`opencode-ai`)

## 📖 문서

한국어 문서는 아래 링크를 참고하세요:

- [CONTEXT 문서 인덱스](CONTEXT/README.md)
- [상세 사용 가이드](CONTEXT/ko/USAGE.md)
- [사용량 알림](CONTEXT/ko/USAGE_ALERTS.md)
- [상시 모니터링](CONTEXT/ko/MONITORING.md)
- [로컬 개발 및 테스트](CONTEXT/ko/LOCAL_TEST.md)

## 요구사항

- **런타임 사용자**
  - **macOS**: Homebrew 및 Node.js/npm
  - **Ubuntu/Linux**: Node.js/npm
  - **Windows**: Node.js/npm (실험적)
- **개발자 (소스 빌드/테스트)**
  - **Go >= 1.25**

## 라이선스

MIT © Suho Han

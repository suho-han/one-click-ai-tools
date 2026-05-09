# one-click-tools (oct)

[![npm version](https://img.shields.io/npm/v/one-click-tools.svg?style=flat-square)](https://www.npmjs.com/package/one-click-tools)
[![pnpm](https://img.shields.io/badge/maintained%20with-pnpm-cc00ff.svg?style=flat-square&logo=pnpm)](https://pnpm.io/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

**one-click-tools (oct)** is a high-performance, OS-aware CLI utility to bootstrap and update popular AI developer tools with a single command.

## 🚀 Quick Start

### Installation

```bash
# Via npm
npm install -g one-click-tools

# Via pnpm
pnpm add -g one-click-tools
```

### Basic Usage

```bash
# Update all AI agents (Claude, Codex, Gemini, Copilot)
oct agent-update

# Check AI tool usage/quota
oct usage

# Show help
oct help
```

## ✅ 쉬운 사용법 (지금 바로 쓰는 명령어)

아래 4가지만 알면 대부분 바로 사용할 수 있습니다.

### 1) 에이전트 업데이트

```bash
# Claude/Codex/Gemini/Copilot 업데이트
oct agent-update
```

### 2) 사용량 확인

```bash
# 현재 사용량 확인
oct usage
```

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

#### 세부 임계치(윈도우/프로바이더별)

```bash
# 전역 윈도우별
oct alert config set threshold.5h 90
oct alert config set threshold.7d 92

# provider별
oct alert config set provider.codex.5h 94
oct alert config set provider.codex.default 88
```

#### 일시 정지(snooze)

```bash
# 전체 알림 2시간 일시정지
oct alert snooze set --duration 2h

# 특정 provider/window만 일시정지
oct alert snooze set --duration 1h --provider codex --window 5h

# 확인/해제
oct alert snooze show
oct alert snooze clear --provider codex --window 5h
```

## 🛠 Supported Agents

- **Claude Code** (`@anthropic-ai/claude-code`)
- **OpenAI Codex** (`@openai/codex`)
- **Gemini CLI** (`@google/gemini-cli`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor**
- **OpenCode**

## 📖 Documentation

For detailed guides, please refer to the `CONTEXT/` directory:

- [Detailed Usage Guide](CONTEXT/USAGE.md)
- [Local Development & Testing](CONTEXT/LOCAL_TEST.md)
- [Project Plan & Status](CONTEXT/PLAN.md)
- [Roadmap & TODO](CONTEXT/TODO.md)
- [Icon Integration](CONTEXT/ICONS.md)
- [Usage Alerts](CONTEXT/USAGE_ALERTS.md)
- [Always-on Monitoring](CONTEXT/MONITORING.md)

## Requirements

- **Runtime users**
  - **macOS**: Homebrew and Node.js/npm
  - **Ubuntu/Linux**: Node.js/npm
  - **Windows**: Node.js/npm (Experimental)
- **Developers (build/test from source)**
  - **Go >= 1.25**

## License

MIT © Suho Han

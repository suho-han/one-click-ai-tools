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

## 🛠 Supported Agents

- **Claude Code** (`@anthropic-ai/claude-code`)
- **OpenAI Codex** (`@openai/codex`)
- **Gemini CLI** (`@google/gemini-cli`)
- **GitHub Copilot** (`@github/copilot`)

## 📖 Documentation

For detailed guides, please refer to the `CONTEXT/` directory:

- [Detailed Usage Guide](CONTEXT/USAGE.md)
- [Local Development & Testing](CONTEXT/LOCAL_TEST.md)
- [Project Plan & Status](CONTEXT/PLAN.md)
- [Roadmap & TODO](CONTEXT/TODO.md)
- [Icon Integration](CONTEXT/ICONS.md)

## Requirements

- **Runtime users**
  - **macOS**: Homebrew and Node.js/npm
  - **Ubuntu/Linux**: Node.js/npm
  - **Windows**: Node.js/npm (Experimental)
- **Developers (build/test from source)**
  - **Go >= 1.22**

## License

MIT © Suho Han

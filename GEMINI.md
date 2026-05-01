# Project: One-Click Tools (oct)

`one-click-tools` (shorthand `oct`) is a high-performance, OS-aware CLI utility written in Go. Its primary purpose is to bootstrap and update popular AI developer tools (like Claude Code, OpenAI Codex, Gemini CLI, and GitHub Copilot) with a single command, supporting macOS, Ubuntu, and Windows.

## Project Overview

- **Core Technology:** Go (v1.26+) using the `cobra` framework for CLI structure and `viper` for configuration management.
- **Distribution:** Distributed via npm (`one-click-tools`) using a Node.js wrapper (`scripts/oct-wrapper.js`) to provide cross-platform binary execution.
- **Concurrency:** Uses Goroutines and `sync/errgroup` to perform parallel updates of multiple AI tools.
- **OS Abstraction:** Provides platform-specific implementations for scheduling and updates (Homebrew on macOS, npm on Linux, Task Scheduler on Windows).

## Key Components

- `main.go`: Entry point for the Go application.
- `cmd/`: Contains CLI command definitions (Root, Agent Update, Usage, Schedule).
- `internal/`: Core business logic:
    - `update/`: Logic for tool updates and package management.
    - `usage/`: Collects and reports usage metrics for various AI services.
    - `schedule/`: OS-specific task scheduling (launchd, cron, schtasks).
    - `config/`: Configuration handling and legacy config migration.
- `scripts/`: npm-related scripts for installation and execution wrapping.

## Building and Running

### Development
- **Build the binary:**
  ```bash
  go build -o oct main.go
  ```
- **Run the project:**
  ```bash
  go run main.go [command]
  ```
- **Run tests:**
  ```bash
  go test ./...
  ```

### Production Distribution
- **Build for release:** Uses GoReleaser (see `.goreleaser.yaml`).
- **NPM Package:** The `package.json` defines the `one-click-tools` command which points to the JS wrapper.

## Development Conventions

- **CLI Framework:** Strictly use `cobra` for adding new commands.
- **Configuration:** Prefer `viper` for all configuration needs. The config file is located at `~/.oct/config.yaml`.
- **Parallelism:** When performing operations on multiple tools, use the patterns established in `internal/update/update.go` with `errgroup`.
- **OS Compatibility:** Always consider cross-platform compatibility. Use `path/filepath` for path manipulations and check `runtime.GOOS` for platform-specific logic.
- **Logging:** Execution logs are typically saved to `~/.oct/logs/`.
- **Package Management:** Use `pnpm` for Node.js dependency management.
- **Release Workflow:** Use `npm run release` (standard-version) to bump versions and update the CHANGELOG. Follow with `git push --follow-tags` and `npm publish`.

## Important Commands (CLI)

- `oct agent-update` (or `oct update`): Updates all supported AI tools.
- `oct usage`: Shows usage statistics for configured AI models.
- `oct schedule`: Manages automatic update schedules.
- `oct config`: Manages tool configuration.

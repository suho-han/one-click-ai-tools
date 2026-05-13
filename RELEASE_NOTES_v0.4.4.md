## v0.4.4 (Draft)

### ✨ Features
- **config:** interactive selector now uses `Enter` to toggle items and adds a final **Confirm** row to save.
- **monitor:** provider icons added with terminal capability fallback handling.
- **ci:** npm publish now runs from the release workflow.

### 🛠 Chore
- **config:** environment-variable overrides now require `OCT_` prefix (e.g. `OCT_ENABLED_TOOLS`), preventing accidental override from generic vars like `ENABLED_TOOLS`.
- **npm:** keep project `.npmrc` as a symlink to home config.

### 📚 Documentation
- Documented the new `oct config` interaction flow (Enter toggle + final Confirm row).
- Documented `OCT_` env override policy in Korean/English READMEs.

### ✅ Verification
- `go test ./cmd/...`
- `GOTOOLCHAIN=auto go test ./...`
- `go build -o oct main.go`

### Commits included since `v0.4.3`
- `70f06a6` docs(config): document Enter+Confirm flow and OCT_ env override
- `48a0796` chore(config): require OCT_ prefix for env overrides
- `cf40914` chore(npm): keep .npmrc linked to home config
- `ae39f7e` feat(config): use Enter toggle and add final Confirm row
- `1ed3e6e` feat(ci): publish npm on release workflow
- `8a3cb47` feat(monitor): add provider icons with terminal capability fallback

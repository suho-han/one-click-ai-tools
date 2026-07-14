# Always-on Monitoring Guide

`oct monitor` refreshes provider usage continuously in a terminal view.

## Basic usage

```bash
oct monitor
oct monitor --interval 5s
oct monitor --once
oct monitor --once --sort-by used --desc --top 5 --compact
```

## Key options

- `--interval`: refresh interval (default 30s)
- `--once`: run one cycle and exit
- `--sort-by provider|used|5h|7d`: sort key
- `--desc`: descending order
- `--top N`: show only top N
- `--compact`: compact output
- `--state-path`: snapshot output path

## Output and snapshots

- Columns: `provider`, `5h`, `7d`, `sev`, `status` (+ `used`, `limit`, `message` in default mode)
- Default snapshot path: `~/.oct/state/usage-latest.json`
- If `usage_display_mode=remaining`, values are shown as remaining quota (`100-used`)

## Operational tips

Linux:
```bash
tmux new -s oct-monitor 'oct monitor --interval 10s'
```

Windows (PowerShell):
```powershell
oct monitor --interval 10s
```

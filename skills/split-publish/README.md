# split-publish skill

A portable prompt skill for splitting mixed git changes into task-based commits and publishing safely.

## Files

- `SKILL.md`: Codex skill format
- `CLAUDE.md`: Claude-friendly instruction block
- `references/grouping-rules.md`: commit boundary rubric

## Install (Codex)

Copy this directory to:

- `~/.codex/skills/split-publish`

Then ensure Codex can discover the skill in your environment.

## Install (Claude)

Use one of these approaches:

1. Add `CLAUDE.md` content to project `AGENTS.md` or your Claude project memory.
2. Keep this folder in your repo and tell Claude to follow `skills/split-publish/CLAUDE.md`.

## Trigger Phrases

- split and push by job
- group changes into separate commits and publish
- split publish

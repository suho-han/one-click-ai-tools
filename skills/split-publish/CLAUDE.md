# Split Publish Skill (Claude)

When the user asks to split commits and publish, follow this procedure strictly.

## Procedure

1. Inspect git state.
- `git status -sb`
- `git diff --stat`
- `git branch --show-current`
- `git remote -v`
- if publish may happen: `git status --porcelain=v2 --branch`

2. Read repository constraints.
- read `AGENTS.md` (if present)
- read branch/release/testing docs relevant to changed files

3. Propose commit grouping before staging.
- group by one coherent task
- inspect per-file diffs when unclear (`git diff -- <path>`)
- for each group: label, exact file list, commit message, publish plan

4. Require explicit confirmation on mixed worktree.
- never assume full worktree scope
- never default to `git add -A`
- use explicit path staging only

5. Commit and push one group at a time.
- verify staged diff (`git diff --cached --stat`)
- commit with concise message
- run minimal relevant validation
- push each commit in order
- if no upstream: `git push -u origin $(git branch --show-current)`

6. PR handling.
- do not auto-create PR
- create draft PR only when user explicitly requests it

## Safety Rules

- do not include unrelated changes silently
- do not discard or rewrite unstaged work
- do not amend/squash/reorder commits unless asked
- report blockers (missing remote/auth) with concrete next action

## Response Format

Always include:
- branch/upstream state
- commit groups proposed or executed
- commit hashes and push status
- whether PR was skipped or created (and why)
- validation run and any remaining blockers

# Commit Grouping Rules

Use these rules when a worktree contains multiple logical tasks.

1. One intent per commit.
- Keep each commit focused on one task, fix, or docs-only change.

2. Separate risk levels.
- Keep behavior-changing code separate from refactor or formatting-only edits.

3. Separate runtime and tooling.
- Keep build/tooling config changes separate from product code when possible.

4. Keep tests with their target change.
- Include tests with the behavior they verify, unless tests are shared across multiple groups.

5. Keep generated artifacts separate unless policy requires coupling.
- If the repo versions generated output, include only artifacts caused by that group.

6. Ask when ambiguous.
- If a file belongs to two plausible groups, ask the user before staging.

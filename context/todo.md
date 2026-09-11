# TODO / Issue Notes

This file tracks forward-looking ideas and follow-up improvements.

## Candidate priorities

- broaden package manager support (`cargo`, `go install`, `pip`, etc.)
- strengthen real-environment E2E automation for Linux/Windows scheduling
- improve Cursor/OpenCode usage visibility (plan/reset timing)
- extend monitor snapshot consumers (tray/widget UI)

## Future usage-provider candidates (researched 2026-09-12)

Endpoints verified against community implementations (CodexBar docs,
opencode-quota); none have an official oct-grade integration yet.

- Alibaba Coding Plan (Qwen/GLM/Kimi/MiniMax bundle): no programmatic quota API
  (console page + 429 codes only; windows would map cleanly to 5h/7d/1m).
  Revisit if an API appears; `sk-sp-` keys.
- Ollama Cloud: `GET https://ollama.com/api/usage` with `OLLAMA_API_KEY`;
  session + weekly usage fractions, no reset timestamps.
- Chutes AI: `GET https://api.chutes.ai/users/me/quota_usage/me` (bearer token).
- NanoGPT: `GET https://nano-gpt.com/api/account` (daily/monthly quota % + USD balance).
- Xiaomi MiMo: `platform.xiaomimimo.com` dashboard API (`/api/v1/balance`) but
  auth is a browser cookie (`api-platform_serviceToken`) -- fragile.
- Augment: `app.augmentcode.com/api/credits` + `/api/subscription`, but auth is
  a browser session cookie; `auggie account status` output parsing is the
  sturdier path.
- Codebuff: `POST https://www.codebuff.com/api/v1/usage` (credit balance +
  reset) and `GET /api/user/subscription` (`weeklyUsed`/`weeklyLimit`);
  Bearer token from `CODEBUFF_API_KEY` or `~/.config/manicode/credentials.json`.
- Amp (Sourcegraph): credits model, no confirmed public quota endpoint.
- Factory (droid): `GET https://api.factory.ai/api/v1/analytics/tokens`
  (Bearer `fk-...` key) reports consumption only -- no remaining/limit surface,
  and per-user scoping is enterprise-gated.

## Maintenance rule

- keep completed history in release notes/PRs; keep this file focused on future work

## Related docs

- [code_review_2026-09-10.md](code_review_2026-09-10.md): detailed improvement backlog snapshot behind some of these items

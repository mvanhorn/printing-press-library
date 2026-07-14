# MetaTrader 5 Printed CLI Agent Guide

This directory is the `mt5-pp-cli` CLI, published in the printing-press-library catalog. Unlike most catalog entries it was **hand-built** by ek-labs following printing-press conventions, not produced by the CLI Printing Press generator — see `.printing-press.json`, `NOTICE`, and `.manuscripts/` for provenance. There is no generator template to fix upstream: defects are fixed in place here and recorded in `.printing-press-patches.json`.

## Local Operating Contract

Start by asking the CLI for current runtime truth:

```bash
mt5-pp-cli doctor
```

`doctor` checks Python, the MetaTrader5 package, terminal state, login, store writability, config, kill-switch state, and safety mode — each failure prints its exact remediation.

Add `--agent` to any command for compact JSON, no color, non-interactive defaults:

```bash
mt5-pp-cli positions list --agent
mt5-pp-cli stats summary --since 30d --agent --select win_rate,profit_factor
```

## Safety semantics — read before any write

Every write command (`order send`, `position close|modify`, `close all`) is dry-run by default and passes through four independent gates. As an agent you must:

1. Compose the command normally (or with `--dry-run`). The first invocation prints a SHA-256 intent hash and exits **6**.
2. Surface the dry-run summary and hash to the human and stop.
3. Re-invoke with `--confirm <hash>` (valid 60–120 s) only after explicit human approval. Live accounts additionally require `--i-understand-this-is-live` **and** `MT5_LIVE=1` in the environment.
4. **Never set `MT5_LIVE=1` yourself.** That is a user-only action.

Exit codes `5` vs `6` matter: `5` = broker rejected (a retry may help); `6` = safety gate rejected (change the command or get approval — do not retry as-is).

Every write attempt — dry-run, confirmed, rejected — is appended to the audit log (`audit.jsonl` + the `audit` table). Do not delete it.

## MCP surface

`mt5-pp-mcp` exposes 18 tools over stdio (`mt5-pp-mcp --list-tools` enumerates them without booting). Every write tool re-enters the same cobra pipeline in-process, so the safety gates apply identically — an MCP client cannot bypass anything the CLI user couldn't.

## Data locations

- Local mirror: `mt5-pp-cli/store.db` under the platform data dir (Windows `%LOCALAPPDATA%`, Linux `~/.local/share`, macOS `~/Library/Application Support`).
- Config + guardrails: `mt5-pp-cli/config.toml` under the platform config dir. `mt5-pp-cli config-init` writes an example.
- Reads scope to one account: `--account <login>` or the most recently synced account. `exit 10` means run `mt5-pp-cli sync all` first.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. `STATUS.md` is the phased build log.

## Local Customizations

If you modify this CLI, record each customization in `.printing-press-patches.json` at this CLI's root (shape documented in the repo-root AGENTS.md): a short `id`, one-sentence `summary`, one-or-two-sentence `reason`, the touched `files`, and optionally a `validated_outcome`. Diffs live in git; the manifest is the index that survives tree replacement.

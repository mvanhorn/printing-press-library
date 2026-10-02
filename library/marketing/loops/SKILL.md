---
name: pp-loops
description: Agent guide for the Loops Printing Press CLI, including safe team selection, previews, and lifecycle checks.
author: Cathryn Lavery
license: Apache-2.0
argument-hint: "<command> [args]"
allowed-tools: "Read Bash"
---

# Loops CLI agent guide

Inspect [COMMANDS.md](COMMANDS.md) for every command's effect, then call the specific command's `--help`. Use the installed binary or `go run ./cmd/loops-pp-cli` from a source checkout.

## Prerequisites: Install the CLI

This skill drives the `loops-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install loops --cli-only
   ```
2. Verify: `loops-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/loops/cmd/loops-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI covers all 68 operations in the [Loops API](https://loops.so/docs/api-reference/intro) and adds agent-safe team verification, lifecycle inventory, and campaign preflight checks. It previews writes with private values redacted and requires explicit execution for sends and changes.

## Safe sequence

1. Run `loops-pp-cli --version` and `loops-pp-cli doctor --dry-run --json`. The dry doctor makes no network request and sends no email.
2. Select a Loops credential through `LOOPS_API_KEY` or private local credential storage. Keep different teams' credentials and profiles separate. For account-specific work, run `team verify --team 'Expected Team' --agent` before reads; read-only checks may use `LOOPS_EXPECTED_TEAM` instead. Every mutation verifies the exact API team again and still requires an explicit `--team` flag.
3. Use `--agent --no-learn --no-cache` and `--select` for minimal JSON output and local retention. `contacts find --agent` omits identity fields by default. Do not place contact values in teach/recall queries, logs, receipts, source, fixtures, or public descriptions.
4. Inspect a mutation's default redacted preview. Use `--execute --team 'Expected Team'` only after reviewing the target and effect. Sends also need `--confirm-send` and an explicit `--idempotency-key`; campaign publication, suppression removal, and deletes need their respective confirmation flags. `--yes` does not bypass these gates.
5. After any uncertain send result, inspect Loops before taking further action. The local journal blocks reusing that team/key automatically. A definite 429 can be retried later with the same key. Never create a fresh key merely to bypass a duplicate warning.

## Useful agent commands

```sh
loops-pp-cli audit lifecycle --team 'Synthetic Team' --agent
loops-pp-cli campaigns preflight --id SYNTHETIC_CAMPAIGN_ID --team 'Synthetic Team' --agent
loops-pp-cli contacts find --email person@example.test --agent --data-source live --no-cache
printf '%s' '{"eventName":"synthetic_event","email":"person@example.test"}' | loops-pp-cli events --stdin --idempotency-key SYNTHETIC_KEY --agent
```

The final command previews a send with synthetic data; it does not send. Real recipient payloads should be supplied through stdin or a private file. The complete API surface includes contacts, lists, segments, campaigns, metrics, event patterns, workflows, transactional email, message content, themes, components, and uploads. Loops does not expose a bulk contacts list: `sync`, `export`, and `tail` exclude contacts. Use `contacts find` with an explicit identifier. Product usage cohorts require another approved source such as PostHog. Keep PostHog and Loops credentials separate and do not infer first use until product events exist.

## Verification

Local tests use mock HTTP servers. Read-only team verification and lifecycle auditing have passed against a live account without recording account details. A controlled test send remains unverified and requires separate explicit approval.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`team verify`** — Verify the selected API team before a change, preview writes without private values, and journal send attempts.

  ```bash
  loops-pp-cli team verify --agent
  ```
- **`audit lifecycle`** — Count communication resources without retrieving contact records.

  ```bash
  loops-pp-cli audit lifecycle --agent
  ```
- **`campaigns preflight`** — Check draft status, audience targeting, message presence, and Guardian findings before publication.

  ```bash
  loops-pp-cli campaigns preflight --latest-draft --agent
  ```

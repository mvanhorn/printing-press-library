---
name: pp-linq
description: "Printing Press CLI for Linq. Community-curated OpenAPI 3.1 blueprint for the Linq Partner API, derived from https://docs.linqapp."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - linq-pp-cli
    install:
      - kind: go
        bins: [linq-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/cmd/linq-pp-cli
---

# Linq — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `linq-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install linq --cli-only
   ```
2. Verify: `linq-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/cmd/linq-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Community-curated OpenAPI 3.1 blueprint for the Linq Partner API, derived from https://docs.linqapp.com/api and cross-checked against github.com/linq-team/linq-go/api.md. The blueprint is generic and contains no RonanRx-specific behavior or PHI examples.

## RonanRx safety commands

Use these private commands before any patient-adjacent automation:

- `linq-pp-cli invite-link` — create an inbound click-to-text front door with routing text and an opaque token/link only.
- `linq-pp-cli welcome-flow` — plan the full inbound-first welcome flow without sending: invite link, secure-link audit, consent audit, safe draft, guarded send, and monitoring.
- `linq-pp-cli send-preflight` — explain whether guarded send would be allowed; no side effects.
- `linq-pp-cli send` — the only intended outbound agent path; refuses cold sends, opted-out recipients, PHI-shaped content, and non-HTTPS links.
- `linq-pp-cli safe-reply-draft` — draft a redacted reply for human review; never sends.
- `linq-pp-cli consent-audit` — summarize local inbound/opt-out evidence for a chat.
- `linq-pp-cli needs-human` — surface risky conversations for human review.
- `linq-pp-cli link-audit` — validate pointer-not-payload URL safety and allowlisted hosts.
- `linq-pp-cli purge` — enforce local encrypted mirror retention.

Raw mutating Linq endpoint mirrors are MCP-hidden; do not bypass these commands for RonanRx patient-adjacent workflows.

## Command Reference

**attachments** — Manage attachments

- `linq-pp-cli attachments delete-an` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli attachments get-metadata` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli attachments pre-upload-afile` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

**capability** — Manage capability

- `linq-pp-cli capability check-imessage` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli capability check-rcscapability` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

**chats** — Manage chats

- `linq-pp-cli chats create-anew` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli chats get-achat-by-id` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli chats list-all` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli chats update-achat` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

**contact-card** — Manage contact card

- `linq-pp-cli contact-card get` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli contact-card setup` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli contact-card update` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

**messages** — Manage messages

- `linq-pp-cli messages delete-amessage-from-system` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli messages edit-the-content-of-amessage-part` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli messages get-amessage-by-id` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

**phone-numbers** — Manage phone numbers

- `linq-pp-cli phone-numbers` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

**phonenumbers** — Manage phonenumbers

- `linq-pp-cli phonenumbers` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

**webhook-events** — Manage webhook events

- `linq-pp-cli webhook-events` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

**webhook-subscriptions** — Manage webhook subscriptions

- `linq-pp-cli webhook-subscriptions create-anew` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli webhook-subscriptions delete-awebhook-subscription` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli webhook-subscriptions get-awebhook-subscription-by-id` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli webhook-subscriptions list-all` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- `linq-pp-cli webhook-subscriptions update-awebhook-subscription` — Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
linq-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `linq-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
linq-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `LINQ_API_V3_API_KEY` as an environment variable.

Run `linq-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  linq-pp-cli contact-card get --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
linq-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
linq-pp-cli feedback --stdin < notes.txt
linq-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/linq-pp-cli/feedback.jsonl`. They are never POSTed unless `LINQ_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LINQ_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
linq-pp-cli profile save briefing --json
linq-pp-cli --profile briefing contact-card get
linq-pp-cli profile list --json
linq-pp-cli profile show briefing
linq-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `linq-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/cmd/linq-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add linq-pp-mcp -- linq-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which linq-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   linq-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `linq-pp-cli <command> --help`.

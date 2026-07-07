---
name: pp-gisis
description: "Authoritative IMO ship particulars on the command line Trigger phrases: `look up IMO`, `GISIS particulars`, `ship particulars by IMO`, `vessel registry lookup`, `flag-hop history for vessel`, `use gisis`, `run gisis`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gisis-pp-cli
    install:
      - kind: go
        bins: [gisis-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/gisis/cmd/gisis-pp-cli
---

# GISIS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gisis-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install gisis --cli-only
   ```
2. Verify: `gisis-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/gisis/cmd/gisis-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you need authoritative IMO ship particulars — name, flag, type, gross tonnage, registered owner, operator, classification society. The local cache makes it a personal vessel index that compounds over time, and 'ship history' surfaces flag/owner changes that matter for sanctions and due diligence. Pair with GFW (vessel events) and AIS Stream (real-time position) via the Vessel MCP orchestrator for a full DD picture.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for real-time vessel position — use AIS Stream instead.
- Do not use this CLI for fishing-activity or port-visit events — use Global Fishing Watch (GFW).
- Do not use this CLI for OFAC sanctions screening — use the OpenSanctions or OFAC SDN adapter in the Vessel MCP orchestrator.
- Do not use this CLI for ship name search in v1 — defer to v0.2; in the meantime, look up the IMO another way.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`ship batch`** — Resolve a list of IMOs in one invocation, honoring the 1 req/2-3s throttle and persisting each to the local cache.

  _When an agent needs particulars for many vessels, this is the polite + persistent path. Single IMO? Use 'ship get'._

  ```bash
  gisis-pp-cli ship batch --imos 9966233,9123456 --json
  ```
- **`ship list`** — Browse vessels you have already fetched, with filters on flag/owner/type and full-text search on name/owner.

  _When you want to find a vessel you saw before, this beats re-fetching from GISIS._

  ```bash
  gisis-pp-cli ship list --owner "ACME" --type Tanker --json
  ```
- **`ship pin`** — Pin vessels for an active deal or story, then refresh only the pinned ones at a chosen cadence.

  _Use this to keep a working set of vessels current without re-fetching the whole cache._

  ```bash
  gisis-pp-cli ship pin 9966233 --label deal-2026-Q2 && gisis-pp-cli ship refresh --pinned --older-than 30d
  ```
- **`ship stale`** — List cached vessels whose particulars haven't been refreshed in N days.

  _Compliance recency: when you need to know which dossier vessels need re-checking._

  ```bash
  gisis-pp-cli ship stale --older-than 30d --pinned --json
  ```

### Maritime due-diligence signals
- **`ship history`** — Show how a vessel's particulars have changed across snapshots — flag, name, owner, operator, classification society, status.

  _Flag-hopping and ownership changes are the textbook sanctions-bypass tells in maritime DD. Use this when you need temporal context, not a snapshot._

  ```bash
  gisis-pp-cli ship history 9966233 --json
  ```
- **`owner fleet`** — List every cached vessel for a given owner string (the Companies module isn't in v1).

  _Counterparty exposure and related-vessel discovery without hitting the deferred Companies module._

  ```bash
  gisis-pp-cli owner fleet "ACME SHIPPING LTD" --like --json
  ```

### Reachability mitigation
- **`auth ping`** — Single fast GET to /Public/SHIPS/Default.aspx; exits 0 if session is live, non-zero if re-login needed.

  _Long batch jobs and unattended cron tasks need a cheap way to know if the session is still alive._

  ```bash
  gisis-pp-cli auth ping
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**ship** — Ship particulars — authoritative IMO vessel registry data (name, flag, type, gross tonnage, ownership, classification society).

- `gisis-pp-cli ship <IMONumber>` — Get ship particulars by IMO number from the IMO Ship and Company Particulars module.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gisis-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Look up an IMO with structured output

```bash
gisis-pp-cli ship get 9966233 --json --select name,flag,type,gross_tonnage,registered_owner
```

Returns the four high-gravity fields needed for a KYC entry.

### Run a nightly batch over a watchlist

```bash
gisis-pp-cli ship batch --file ~/watchlist.txt --json | jq -c '.'
```

Resolves every IMO in the file under the throttle and streams JSON-lines.

### Spot a flag change on a pinned vessel

```bash
gisis-pp-cli ship refresh --pinned --older-than 7d && gisis-pp-cli ship history 9966233 --json
```

Refresh stale pinned ships, then check the diff on one of them.

### Find all cached ships under one owner

```bash
gisis-pp-cli owner fleet "COSCO SHIPPING" --like --json
```

Synthesizes counterparty exposure from accumulated lookups.

### Keep the session alive from cron

```bash
*/20 * * * * /usr/local/bin/gisis-pp-cli auth ping >/dev/null 2>&1
```

ASP.NET sessions time out at ~30 min idle; ping every 20 to stay warm.

## Auth Setup

GISIS requires a free IMO Web Accounts login (https://webaccounts.imo.org/Register.aspx?App=GISISPublic) with a Cloudflare Turnstile challenge. Programmatic login is blocked, so this CLI uses the press-auth companion: install it once, run 'press-auth login gisis.imo.org', a controlled Chrome window opens for you to sign in, and your cookies are captured into the macOS keychain. The CLI then reads cookies on demand. Sessions die after ~30 min of inactivity; run 'gisis-pp-cli auth ping' from cron to keep them warm, or re-run 'press-auth login' when the session expires.

Run `gisis-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gisis-pp-cli ship mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
gisis-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gisis-pp-cli feedback --stdin < notes.txt
gisis-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/gisis-pp-cli/feedback.jsonl`. They are never POSTed unless `GISIS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GISIS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gisis-pp-cli profile save briefing --json
gisis-pp-cli --profile briefing ship mock-value
gisis-pp-cli profile list --json
gisis-pp-cli profile show briefing
gisis-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gisis-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/gisis/cmd/gisis-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add gisis-pp-mcp -- gisis-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gisis-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gisis-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gisis-pp-cli <command> --help`.

---
name: pp-tidycal
description: "Printing Press CLI for Tidycal. TidyCal's REST API provides a handful of endpoints which can be used to get information about your account and bookings."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tidycal-pp-cli
    install:
      - kind: go
        bins: [tidycal-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/tidycal/cmd/tidycal-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/tidycal/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Tidycal — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tidycal-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install tidycal --cli-only
   ```
2. Verify: `tidycal-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/tidycal/cmd/tidycal-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

# Introduction
TidyCal's REST API provides a handful of endpoints which can be used to get information about your account and bookings.  It uses conventional OAuth 2.0 protocol for authentication.

# Authentication
### Personal Access Token
Create a personal access token at https://tidycal.com/integrations/oauth.  Once created, it can be used to authenticate requests by passing it in the `Authorization` header.
```
Authorization: Bearer {TOKEN}
```

### OAuth 2.0 Client
If you're building a custom integration to TidyCal which requires users to authenticate in order to get access tokens to make API requests on their behalf, you'll need to create an OAuth 2.0 client. This is easy to do from the \"OAuth Apps\" settings page found here https://tidycal.com/integrations/oauth

Using the `authorization_code` grant type to authenticate users using OAuth 2.0 to retrieve an access token is fairly conventional, more information on that process can be found here: https://www.oauth.com/oauth2-servers/server-side-apps/authorization-code/


* Authorization URL: https://tidycal.com/oauth/authorize
* Access Token URL: https://tidycal.com/oauth/token

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agentic scheduling
- **`brief`** — Produce a contact-aware schedule brief for a day or date range.

  _Use this before drafting prep notes or deciding whether a booking needs attention._

  ```bash
  tidycal-pp-cli brief --date today --format json
  ```
- **`triage`** — Identify booking problems such as missing meeting URLs, duplicate contacts, off-hours bookings, cancelled bookings, and incomplete contact data.

  _Use this to decide which bookings require operator review._

  ```bash
  tidycal-pp-cli triage --from today --to +7d --format json
  ```
- **`propose-times`** — Fetch available TidyCal timeslots for a booking type and rank a paste-ready shortlist by date window, timezone, preference, and weekend policy.

  _Use this when proposing meeting options in chat or email._

  ```bash
  tidycal-pp-cli propose-times 123 --from today --to +14d --count 3 --format json
  ```
- **`followups`** — Create an AI-ready follow-up queue from recent bookings without sending messages or notifications.

  _Use this to prepare follow-up tasks after recent meetings._

  ```bash
  tidycal-pp-cli followups --from -7d --to today --format json
  ```
- **`assisted-book`** — Book on behalf of a contact through an inspectable dry-run and explicit confirmation gate.

  _Use this only when the operator has approved the booking details._

  ```bash
  tidycal-pp-cli assisted-book 123 --name Ada --email ada@example.com --slot 2026-06-02T15:00:00Z --dry-run --format json
  ```

## Command Reference

**booking-types** — Manage booking types

- `tidycal-pp-cli booking-types create` — Create a new booking type.
- `tidycal-pp-cli booking-types list` — Get a list of booking types.

**bookings** — Manage bookings

- `tidycal-pp-cli bookings get` — Get a booking by ID.
- `tidycal-pp-cli bookings list` — Get a list of bookings.

**contacts** — Manage contacts

- `tidycal-pp-cli contacts create` — Create a new contact.
- `tidycal-pp-cli contacts list` — Get a list of contacts.

**me** — Manage me

- `tidycal-pp-cli me` — Get account details.

**teams** — Manage teams

- `tidycal-pp-cli teams get` — Get details of a specific team.
- `tidycal-pp-cli teams list` — Get a list of teams the authenticated user has access to.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tidycal-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `tidycal-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
tidycal-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `TIDYCAL_API_TOKEN` as an environment variable.

Run `tidycal-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tidycal-pp-cli booking-types list --agent --select id,name,status
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
tidycal-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tidycal-pp-cli feedback --stdin < notes.txt
tidycal-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/tidycal-pp-cli/feedback.jsonl`. They are never POSTed unless `TIDYCAL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TIDYCAL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tidycal-pp-cli profile save briefing --json
tidycal-pp-cli --profile briefing booking-types list
tidycal-pp-cli profile list --json
tidycal-pp-cli profile show briefing
tidycal-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tidycal-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/tidycal/cmd/tidycal-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tidycal-pp-mcp -- tidycal-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tidycal-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tidycal-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tidycal-pp-cli <command> --help`.

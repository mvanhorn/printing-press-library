---
name: pp-breezedoc
description: "Printing Press CLI for Breezedoc. BreezeDoc's REST API provides a handful of endpoints which can be used to get information about your account and"
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - breezedoc-pp-cli
    install:
      - kind: go
        bins: [breezedoc-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/breezedoc/cmd/breezedoc-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/breezedoc/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Breezedoc — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `breezedoc-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install breezedoc --cli-only
   ```
2. Verify: `breezedoc-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/breezedoc/cmd/breezedoc-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

# Introduction
BreezeDoc's REST API provides a handful of endpoints which can be used to get information about your account and bookings.  It uses conventional OAuth 2.0 protocol for authentication.

# Authentication
### Personal Access Token
Create a personal access token at https://breezedoc.com/integrations/api.  Once created, it can be used to authenticate requests by passing it in the `Authorization` header.
```
Authorization: Bearer {TOKEN}
```

### OAuth 2.0 Client
If you're building a custom integration to BreezeDoc which requires users to authenticate in order to get access tokens to make API requests on their behalf, you'll need to create an OAuth 2.0 client. This is easy to do from the \"OAuth Apps\" settings page found here https://breezedoc.com/integrations/api

Using the `authorization_code` grant type to authenticate users using OAuth 2.0 to retrieve an access token is fairly conventional, more information on that process can be found here: https://www.oauth.com/oauth2-servers/server-side-apps/authorization-code/


* Authorization URL: https://breezedoc.com/oauth/authorize
* Access Token URL: https://breezedoc.com/oauth/token

# Rate Limiting
The API currently has a rate limit of 60 requests per minute. If you exceed this limit, you will receive a 429 error.

## Command Reference

**documents** — Manage documents

- `breezedoc-pp-cli documents get` — Get a specific document
- `breezedoc-pp-cli documents list` — Get list of documents
- `breezedoc-pp-cli documents store` — Create a new document

**invoices** — Manage invoices

- `breezedoc-pp-cli invoices create` — Creates a new invoice with line items. Optionally sends the invoice immediately by setting `send: true`.
- `breezedoc-pp-cli invoices delete` — Deletes a draft invoice. Cannot delete invoices that have already been sent.
- `breezedoc-pp-cli invoices get` — Get a specific invoice
- `breezedoc-pp-cli invoices list` — Retrieves a paginated list of invoices for the authenticated user.
- `breezedoc-pp-cli invoices patch` — Same as PUT - updates an existing invoice. Cannot update invoices with status paid, uncollectible, or void.
- `breezedoc-pp-cli invoices update` — Updates an existing invoice. Cannot update invoices with status: paid, uncollectible, or void.

**me** — Manage me

- `breezedoc-pp-cli me` — Get current user information

**recipients** — Manage recipients

- `breezedoc-pp-cli recipients` — Get list of recipients

**teams** — Manage teams


**templates** — Manage templates

- `breezedoc-pp-cli templates get` — Get a specific template
- `breezedoc-pp-cli templates list` — Get list of templates


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
breezedoc-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `breezedoc-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
breezedoc-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `BREEZEDOC_API_TOKEN` as an environment variable.

Run `breezedoc-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  breezedoc-pp-cli documents list --agent --select id,name,status
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
breezedoc-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
breezedoc-pp-cli feedback --stdin < notes.txt
breezedoc-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/breezedoc-pp-cli/feedback.jsonl`. They are never POSTed unless `BREEZEDOC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BREEZEDOC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
breezedoc-pp-cli profile save briefing --json
breezedoc-pp-cli --profile briefing documents list
breezedoc-pp-cli profile list --json
breezedoc-pp-cli profile show briefing
breezedoc-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `breezedoc-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/breezedoc/cmd/breezedoc-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add breezedoc-pp-mcp -- breezedoc-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which breezedoc-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   breezedoc-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `breezedoc-pp-cli <command> --help`.

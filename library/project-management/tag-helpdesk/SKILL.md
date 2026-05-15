---
name: pp-tag-helpdesk
description: "Printing Press CLI for Tag Helpdesk. Odoo 18 CE helpdesk ticket management for tag.msg.it. Connects to the Odoo XML-RPC external API at /xmlrpc/2/common..."
author: "Andrea M. Piovesana"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tag-helpdesk-pp-cli
---

# Tag Helpdesk — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tag-helpdesk-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install tag-helpdesk --cli-only
   ```
2. Verify: `tag-helpdesk-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Odoo 18 CE helpdesk ticket management for tag.msg.it.

Connects to the Odoo XML-RPC external API at /xmlrpc/2/common (auth)
and /xmlrpc/2/object (ORM). Syncs helpdesk.ticket records into a local
SQLite cache for offline analysis and Claude-native pipelines.

Auth: set ODOO_URL, ODOO_DB, ODOO_USER, ODOO_API_KEY environment variables.
Generate an API key in Odoo via Settings → Users → your user → Account Security.

## Command Reference

**xmlrpc** — Manage xmlrpc

- `tag-helpdesk-pp-cli xmlrpc authenticate` — Calls common.authenticate(db, username, api_key, {}) via XML-RPC. Returns integer UID used in all subsequent object...
- `tag-helpdesk-pp-cli xmlrpc count-tickets` — Count tickets matching a domain
- `tag-helpdesk-pp-cli xmlrpc create-ticket` — Create a new helpdesk ticket
- `tag-helpdesk-pp-cli xmlrpc get-ticket` — Calls execute_kw(db, uid, api_key, 'helpdesk.ticket', 'read', [[id]], {fields}).
- `tag-helpdesk-pp-cli xmlrpc get-ticket-messages` — Calls execute_kw on mail.message with domain [('res_model','=','helpdesk.ticket'),('res_id','=',id)]. Returns...
- `tag-helpdesk-pp-cli xmlrpc list-categories` — List ticket categories
- `tag-helpdesk-pp-cli xmlrpc list-stages` — List ticket stages
- `tag-helpdesk-pp-cli xmlrpc list-tags` — List ticket tags
- `tag-helpdesk-pp-cli xmlrpc list-teams` — List helpdesk teams
- `tag-helpdesk-pp-cli xmlrpc list-tickets` — Calls execute_kw(db, uid, api_key, 'helpdesk.ticket', 'search_read', [domain], {fields, limit, offset, order})....
- `tag-helpdesk-pp-cli xmlrpc post-note` — Post an internal note on a ticket
- `tag-helpdesk-pp-cli xmlrpc update-ticket` — Update ticket fields


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tag-helpdesk-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `tag-helpdesk-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export TAG_HELPDESK_API_KEY="<your-key>"
```

Or persist it in `~/.config/tag-helpdesk-pp-cli/config.toml`.

Run `tag-helpdesk-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tag-helpdesk-pp-cli xmlrpc authenticate --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tag-helpdesk-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tag-helpdesk-pp-cli feedback --stdin < notes.txt
tag-helpdesk-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.tag-helpdesk-pp-cli/feedback.jsonl`. They are never POSTed unless `TAG_HELPDESK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TAG_HELPDESK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tag-helpdesk-pp-cli profile save briefing --json
tag-helpdesk-pp-cli --profile briefing xmlrpc authenticate
tag-helpdesk-pp-cli profile list --json
tag-helpdesk-pp-cli profile show briefing
tag-helpdesk-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tag-helpdesk-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add tag-helpdesk-pp-mcp -- tag-helpdesk-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tag-helpdesk-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tag-helpdesk-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tag-helpdesk-pp-cli <command> --help`.

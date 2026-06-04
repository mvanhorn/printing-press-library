---
name: pp-midjourney
description: "Inspect Midjourney jobs, queue, folders, and discovery feeds from the terminal"
author: "Dave Fano"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - midjourney-pp-cli
    install:
      - kind: go
        bins: [midjourney-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/midjourney/cmd/midjourney-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/ai/midjourney/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Midjourney — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `midjourney-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install midjourney --cli-only
   ```
2. Verify: `midjourney-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/midjourney/cmd/midjourney-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Unofficial browser-session CLI for Midjourney's web app API.

## When Not to Use This CLI

Most generated endpoint commands are read-only inspection commands. Do not use mutating commands unless the user explicitly asks to submit Midjourney work. In particular, `imagine` and `rerun` create remote Midjourney jobs, and `download` writes a local file.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 62 API entries from 62 total network entries
- Protocols: rest_json (75% confidence)
- Generation hints: browser_http_transport, requires_protected_client
- Candidate command ideas: list_contests_ranking_count — Derived from observed GET /api/contests-ranking-count traffic.; list_editor_sessions_sync — Derived from observed GET /api/editor-sessions-sync traffic.; list_explore — Derived from observed GET /api/explore traffic.; list_explore_styles_likes — Derived from observed GET /api/explore-styles-likes traffic.; list_folders — Derived from observed GET /api/folders traffic.; list_following_for_user — Derived from observed GET /api/following-for-user traffic.; list_get_user_country — Derived from observed GET /api/get-user-country traffic.; list_imagine — Derived from observed GET /api/imagine traffic.

## Command Reference

**explore** — Browse Midjourney explore feeds

- `midjourney-pp-cli explore list` — Fetch an explore feed
- `midjourney-pp-cli explore style-likes` — Fetch style-like metadata for explore cards

**folders** — Inspect Midjourney organize folders

- `midjourney-pp-cli folders` — List folders in the authenticated account

**generations** — Inspect generated Midjourney jobs and image history

- `midjourney-pp-cli generations list` — List recent jobs/images for the authenticated Midjourney user
- `midjourney-pp-cli generations updates` — Poll recent job updates for the authenticated user

**moodboards** — Inspect Midjourney moodboards

- `midjourney-pp-cli moodboards` — List moodboards for the authenticated account

**profiles** — Inspect personalization/profile metadata

- `midjourney-pp-cli profiles following` — List following metadata for the logged-in user
- `midjourney-pp-cli profiles personalized` — List personalized profile metadata

**queue** — Inspect the current Midjourney generation queue

- `midjourney-pp-cli queue` — Show queued/running Midjourney work for the logged-in account

**rankings** — Inspect ranking and rating surfaces

- `midjourney-pp-cli rankings contests-count` — Fetch contest ranking counts
- `midjourney-pp-cli rankings model-ratings` — Fetch model rating tasks visible to the account

**storage** — Inspect Midjourney account storage metadata

- `midjourney-pp-cli storage` — List storage metadata exposed by the web app


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
midjourney-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `midjourney-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export MIDJOURNEY_COOKIE_HEADER="<your-key>"
```

Or persist it in ``.

Run `midjourney-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  midjourney-pp-cli explore list --agent --select id,name,status
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
midjourney-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
midjourney-pp-cli feedback --stdin < notes.txt
midjourney-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.midjourney-pp-cli/feedback.jsonl`. They are never POSTed unless `MIDJOURNEY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MIDJOURNEY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
midjourney-pp-cli profile save briefing --json
midjourney-pp-cli --profile briefing explore list
midjourney-pp-cli profile list --json
midjourney-pp-cli profile show briefing
midjourney-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `midjourney-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/midjourney/cmd/midjourney-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add midjourney-pp-mcp -- midjourney-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which midjourney-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   midjourney-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `midjourney-pp-cli <command> --help`.

---
name: pp-peloton
description: "Printing Press CLI for Peloton. Read-only Peloton workout, class, and structural-provider facts in a private local store."
author: "Felix Banuchi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - peloton-pp-cli
    install:
      - kind: go
        bins: [peloton-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/peloton/cmd/peloton-pp-cli
---

# Peloton — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `peloton-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install peloton --cli-only
   ```
2. Verify: `peloton-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/peloton/cmd/peloton-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only Peloton workout, class, and structural-provider facts in a private local store.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**account** — Current account/profile fact; no implicit account expansion.

- `peloton-pp-cli account` — Show the current profile fact.

**auth** — Manage Peloton credentials. No OAuth provisioning service is involved; this just wraps the automatic-login lifecycle (see Auth Setup below).

- `peloton-pp-cli auth setup` — Show how to supply your Peloton login for automatic sign-in.
- `peloton-pp-cli auth status` — Show whether a bearer token and/or session cookie are currently available.
- `peloton-pp-cli auth logout` — Remove persisted Peloton credentials.

**classes** — Read-only catalog, class detail, planned structure, and provider filter vocabulary.

- `peloton-pp-cli classes catalog` — List a caller-scoped archived class catalog page.
- `peloton-pp-cli classes filters` — Show provider class/filter vocabulary and embedded instructor metadata.
- `peloton-pp-cli classes search` — Search the caller-scoped catalog by factual provider filters.
- `peloton-pp-cli classes show` — Show class metadata and supported planned structure.
- `peloton-pp-cli classes structure` — Inspect ordered provider segments and target ranges without coaching labels.

**doctor** — Check configuration, credential, and API-connectivity health.

- `peloton-pp-cli doctor` — Report auth state, credential location, API reachability, and local sync cache freshness.

**offline** — Inspect locally synced provider facts with no network access.

- `peloton-pp-cli offline history` — List locally stored recorded workout facts.
- `peloton-pp-cli offline workout <workout_id>` — Show a locally stored workout detail and its recorded history fact.
- `peloton-pp-cli offline performance <workout_id>` — Show locally stored recorded performance samples for one workout.
- `peloton-pp-cli offline intervals <workout_id>` — Show the stored class segments associated with a recorded workout, when available.
- `peloton-pp-cli offline classes search` — Search local class facts by stored fields and structural predicates.
- `peloton-pp-cli offline classes show <ride_id>` — Show one locally stored class fact.
- `peloton-pp-cli offline classes structure <ride_id>` — Show ordered stored class segments and target fields.
- `peloton-pp-cli offline classes filters` — Show locally stored provider filter vocabulary.
- `peloton-pp-cli offline strength <workout_id>` — Show stored movement-tracker fields for one workout.
- `peloton-pp-cli offline repeat <first_workout_id> <second_workout_id>` — Compare two recorded workouts, only when their stored class identifiers match.

**strength** — Provider-supplied performed movement facts present only in workout detail payloads.

- `peloton-pp-cli strength <workout_id>` — Inspect provider workout detail containing movement_tracker_data when present; no template fallback.

**sync** — Sync API data to local SQLite for offline search and analysis.

- `peloton-pp-cli sync` — Sync the default resources (`workouts`, `classes`). Naming `workouts` also cascades into per-workout `performance` samples and `workout_details` payloads (no bulk endpoint exists for those; one request per workout each), which back `offline workout`/`intervals`/`repeat`/`strength`. Naming `classes` also cascades into `classes_detail`: the bulk classes catalog endpoint never returns `segments`/`target_metrics_data`, only the per-class detail endpoint does, so this fetches detail for the classes you've actually taken (via `workouts.ride_id`, not the full catalog) to populate what `offline classes structure`/`offline intervals` read.
- `peloton-pp-cli sync --resources <list>` — Sync specific resources: `workouts`, `classes`, `performance`, `workout_details`, or `classes_detail` (`strength` is accepted as an alias for `workout_details`).
- `peloton-pp-cli sync --resources performance --full --max-parents <n>` — Bound and resume a per-workout dependent backfill (performance/workout_details/classes_detail have no bulk endpoint). Default (no `--full`) only fetches records missing the dependent's content, so repeated calls drain a large backlog for free; `--full` redoes everything (e.g. to backfill a fix) and resumes across calls via a persisted offset; `--max-parents` caps how much happens per call.
- `peloton-pp-cli sync --resources performance --stale-before <timestamp|duration> --max-parents <n>` — Targeted alternative to `--full`: refetch only records last fetched before the given cutoff (RFC3339 timestamp or a `--since`-style duration like `7d`), skipping already-correct records instead of walking the whole backlog.

**workouts** — Read-only recorded workout history, detail, and recorded performance facts.

- `peloton-pp-cli workouts list` — List workout history in newest-first pages; `user_id` must be supplied explicitly (no account-linking shortcut yet).
- `peloton-pp-cli workouts performance` — Show recorded performance samples and summaries for one workout.
- `peloton-pp-cli workouts show` — Show a recorded workout detail payload.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
peloton-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Peloton has no OAuth provisioning service — this CLI just needs your Peloton login. Set it once:

```bash
export PELOTON_OAUTH_USERNAME="your-peloton-email-or-username"
export PELOTON_OAUTH_PASSWORD="your-peloton-password"
```

The first live command logs in automatically and persists the resulting credentials to `~/.config/peloton-pp-cli/oauth-token.json`; later commands reuse or refresh them without the env vars set.

Run `peloton-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  peloton-pp-cli classes search --browse-category example-value --content-format example-value --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `PELOTON_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PELOTON_CONFIG_DIR`, `PELOTON_DATA_DIR`, `PELOTON_STATE_DIR`, `PELOTON_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PELOTON_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains `config.toml`, saved profiles, and the managed auth bundle (`oauth-token.json` — see Auth Setup above). `data` contains `data.db` (the local sync/offline store) and `feedback.jsonl`. `state` is resolved but not currently used by this CLI. `cache` contains the regenerable HTTP response cache.
- Peloton's managed auth persists to `oauth-token.json` under the config dir automatically (see Auth Setup above) — not `credentials.toml` under the data dir. That generic credentials-file mechanism exists in the underlying framework but this CLI's real login flow never writes to it.
- Run `peloton-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "peloton": {
        "command": "peloton-pp-mcp",
        "env": {
          "PELOTON_HOME": "/srv/peloton"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PELOTON_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PELOTON_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
peloton-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
peloton-pp-cli feedback --stdin < notes.txt
peloton-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PELOTON_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PELOTON_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
peloton-pp-cli profile save briefing --json
peloton-pp-cli --profile briefing classes search --browse-category example-value --content-format example-value
peloton-pp-cli profile list --json
peloton-pp-cli profile show briefing
peloton-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `peloton-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/health/peloton/cmd/peloton-pp-mcp@latest
   ```
2. The MCP server needs the same Peloton login as the CLI (see Auth Setup above) — it does not inherit a shell environment automatically, so pass the credentials through explicitly: either export them before registering, or set them in the registered server's `env` block (`claude mcp add --help` shows the exact flag for your Claude Code version). Without this, every tool call fails with an actionable "credentials unavailable" error instead of silently working.
3. Register with Claude Code:
   ```bash
   claude mcp add peloton-pp-mcp -- peloton-pp-mcp
   ```
4. Verify: `claude mcp list`

For Claude Desktop (not Claude Code), use the prebuilt `.mcpb` bundle instead — see "Use with Claude Desktop" in README.md.

## Direct Use

1. Check if installed: `which peloton-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Command Reference above, or run `peloton-pp-cli which "<capability>"` (see Finding the right command above).
3. Execute with the `--agent` flag:
   ```bash
   peloton-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `peloton-pp-cli <command> --help`.

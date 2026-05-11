---
name: pp-yc-companies
description: "Every YC-backed company in a local database, with watch lists, deltas, and cross-batch stats no scraper has. Trigger phrases: `find YC companies`, `search YC startups`, `what changed in YC`, `YC peers of`, `stats across YC batches`, `use yc-companies`, `run yc-companies`."
author: "nickpuru"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - yc-companies-pp-cli
---

# Y Combinator — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `yc-companies-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install yc-companies --cli-only
   ```
2. Verify: `yc-companies-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync the full Y Combinator company directory into a local SQLite store, then filter across batch, industry, tag, status, region, and team size in one query. Watch a portfolio and see what changed between syncs. Compute cross-batch aggregates the live site cannot show.

## When to Use This CLI

Pick this CLI when an agent or analyst needs to filter, search, or compute aggregates across the Y Combinator portfolio without round-tripping through a browser or rewriting the same pandas script. The local SQLite cache makes every cross-axis filter sub-second; the snapshot table makes 'what changed' questions answerable; the tag-Jaccard similar command finds peers without an LLM-judged comparison.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch add`** — Track a personal set of YC companies and see what changes between syncs.

  _Reach for this when an agent needs to compare a list of YC companies across syncs without re-uploading the slug list each time._

  ```bash
  yc-companies-pp-cli watch add stripe airbnb doordash
  ```
- **`watch diff`** — Show team_size, status, and hiring changes on your watched companies since a prior snapshot.

  _Reach for this when an agent is asked 'what changed on these YC companies recently' and needs structured delta rows, not a fresh full list._

  ```bash
  yc-companies-pp-cli watch diff --since 2026-04-01 --agent
  ```
- **`companies new`** — Companies that appeared in the directory after a date or since the last sync.

  _Reach for this for 'what's new in YC' questions — the only correct answer requires snapshot history._

  ```bash
  yc-companies-pp-cli companies new --since 2026-04-01 --json --select slug,name,batch,one_liner
  ```
- **`companies changes`** — Diff status, team_size, or isHiring across the whole index between two snapshots, optionally scoped to specific slugs or target values.

  _Reach for this for any 'who flipped X' question — newly hiring, newly acquired, jumped team size._

  ```bash
  yc-companies-pp-cli companies changes --field isHiring --to true --since 2026-04-01 --json
  ```

### Cross-row local computation
- **`companies similar`** — Given a YC slug, rank peers by tag overlap, industry match, and batch proximity.

  _Reach for this when an agent needs competitors or peers for a specific YC company without an LLM-judged similarity call._

  ```bash
  yc-companies-pp-cli companies similar stripe --limit 10 --json --select slug,name,score,shared_tags
  ```
- **`stats by-batch`** — GROUP BY over the local companies table — count, average team size, % hiring, % top, % acquired per cell.

  _Reach for this for any 'how has X changed across batches' question — counts, growth, hiring share, team-size trends._

  ```bash
  yc-companies-pp-cli stats by-batch --industry fintech --json
  ```
- **`batches show`** — One-shot batch view: company count, top industries, top tags, % hiring, % top company, % acquired, median team size.

  _Reach for this when an agent or analyst needs a quick read on the shape of a single batch._

  ```bash
  yc-companies-pp-cli batches show w25 --json
  ```

## Command Reference

**batches** — Browse YC batches.

- `yc-companies-pp-cli batches <batch_slug>` — Fetch every company in a YC batch.

**companies** — List, fetch, filter, and search Y Combinator companies.

- `yc-companies-pp-cli companies black_founded` — Fetch Black-founded YC companies.
- `yc-companies-pp-cli companies get_in_batch` — Fetch a single company by batch slug and company slug.
- `yc-companies-pp-cli companies hiring` — Fetch companies currently hiring.
- `yc-companies-pp-cli companies hispanic_latino_founded` — Fetch Hispanic/Latino-founded YC companies.
- `yc-companies-pp-cli companies list` — Fetch the full directory (5,889+ companies).
- `yc-companies-pp-cli companies nonprofit` — Fetch nonprofit YC companies.
- `yc-companies-pp-cli companies top` — Fetch top-ranked YC companies.
- `yc-companies-pp-cli companies women_founded` — Fetch women-founded YC companies.

**industries** — Browse YC industry verticals.

- `yc-companies-pp-cli industries <industry_slug>` — Fetch every company in an industry.

**meta** — Directory metadata.

- `yc-companies-pp-cli meta` — Counts and last-updated timestamp for the directory.

**tags** — Browse YC tags.

- `yc-companies-pp-cli tags <tag_slug>` — Fetch every company with a given tag.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
yc-companies-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### AI companies hiring in SF this batch

```bash
yc-companies-pp-cli companies list --batch w25 --tag ai --hiring --region 'United States' --json --select slug,name,one_liner,team_size,website
```

Multi-axis filter with a focused projection — the YC site can't compose these dimensions in one URL.

### What's new since last Monday

```bash
yc-companies-pp-cli companies new --since 2026-04-29 --json --select slug,name,batch,one_liner,launched_at
```

Anti-join across snapshots returns rows present now and absent on the given date.

### Companies that started hiring this month

```bash
yc-companies-pp-cli companies changes --field isHiring --to true --since 2026-04-01 --json --select slug,name,batch,team_size
```

Hiring-flip detection across snapshots — the recruiter ritual collapsed to one command.

### Peers of Stripe by tag overlap

```bash
yc-companies-pp-cli companies similar stripe --limit 10 --json --select slug,name,score,shared_tags,batch
```

Local Jaccard ranking — no LLM, deterministic, sub-second.

### Fintech growth across batches

```bash
yc-companies-pp-cli stats by-batch --industry fintech --json
```

Cross-batch pivot — count, avg team_size, % hiring, % top per batch.

## Auth Setup

No authentication required.

Run `yc-companies-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  yc-companies-pp-cli batches mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
yc-companies-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
yc-companies-pp-cli feedback --stdin < notes.txt
yc-companies-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.yc-companies-pp-cli/feedback.jsonl`. They are never POSTed unless `YC_COMPANIES_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YC_COMPANIES_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
yc-companies-pp-cli profile save briefing --json
yc-companies-pp-cli --profile briefing batches mock-value
yc-companies-pp-cli profile list --json
yc-companies-pp-cli profile show briefing
yc-companies-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `yc-companies-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add yc-companies-pp-mcp -- yc-companies-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which yc-companies-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   yc-companies-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `yc-companies-pp-cli <command> --help`.

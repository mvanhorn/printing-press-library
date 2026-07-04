---
name: pp-sumble
description: "Every Sumble v6 feature, plus the credit-awareness the API itself won't give you Trigger phrases: `find companies using a technology on sumble`, `estimate the sumble credit cost`, `check my sumble credit balance`, `enrich a company's tech stack`, `find engineering leaders at a company`, `use sumble`, `run sumble`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sumble-pp-cli
---

# Sumble — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sumble-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install sumble --cli-only
   ```
2. Verify: `sumble-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/sumble/cmd/sumble-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when an agent or operator needs to prospect or enrich leads through Sumble while keeping credit spend under tight control. It is the right tool when cost predictability matters: previewing spend, enforcing a budget, and reusing a local cache instead of re-billing. It is also the only programmatic way to read the Sumble credit balance, which the REST API does not expose.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Credit economy
- **`cost-estimate`** — See exactly how many credits a billed Sumble call will cost before you run it.

  _Reach for this before any find/enrich/brief call so you never spend credits blind; the estimate is exact, not heuristic._

  ```bash
  sumble-pp-cli cost-estimate organizations.find --rows 25
  ```
- **`balance`** — Show remaining Sumble credits and recent burn without spending any.

  _Use this to check headroom before a batch of billed calls — it is the only programmatic way to read the balance._

  ```bash
  sumble-pp-cli balance --json
  ```
- **`budget`** — Set a credit ceiling so any call whose estimate exceeds it is refused before it dials.

  _Set this once at the start of an autonomous session; billed commands then self-abort with a nonzero exit instead of overspending._

  ```bash
  sumble-pp-cli budget set 500
  ```
- **`spend`** — Break down credits spent over time by endpoint and by day.

  _Use this to find what is eating credits (usually people enrich and intelligence briefs) and adjust the workflow._

  ```bash
  sumble-pp-cli spend --since 2026-05-01 --by endpoint
  ```

### Local-cache leverage
- **`stale`** — List cached organizations, people, and jobs older than Sumble's freshness window.

  _Run before a refresh pass to re-bill only stale rows instead of the whole cache._

  ```bash
  sumble-pp-cli stale --older-than 24h --json
  ```
- **`stack-diff`** — Compare two cached organizations' technology stacks — shared and unique technologies.

  _Use this for competitive teardown or to find a prospect's gaps versus a reference account, without spending credits._

  ```bash
  sumble-pp-cli stack-diff stripe.com adyen.com
  ```

### Cheap-path workflows
- **`reconcile`** — Resolve a CSV of company names/URLs to Sumble IDs via the cheap match endpoint, then report which still need a billed enrich.

  _Use this to attach Sumble IDs to a CRM export at minimal cost before deciding which accounts justify a full enrich._

  ```bash
  sumble-pp-cli reconcile accounts.csv --json
  ```

## Command Reference

**contact-lists** — Manage saved contact (people) lists

- `sumble-pp-cli contact-lists add` — Add people to a saved contact list by id (free)
- `sumble-pp-cli contact-lists create` — Create a new contact list (free)
- `sumble-pp-cli contact-lists get` — Get the people in a saved contact list (1 credit per person returned)
- `sumble-pp-cli contact-lists list` — List your saved contact lists (1 credit per list returned)

**organization-lists** — Manage saved organization lists

- `sumble-pp-cli organization-lists add` — Add organizations to a saved list by id or slug (free)
- `sumble-pp-cli organization-lists create` — Create a new organization list (free)
- `sumble-pp-cli organization-lists get` — Get the organizations in a saved list (1 credit per organization returned)
- `sumble-pp-cli organization-lists list` — List your saved organization lists (1 credit per list returned)

**organizations** — Find, enrich, and match organizations by technographic and firmographic criteria

- `sumble-pp-cli organizations enrich` — Enrich one organization's technology stack with job/people/team signals (5 credits per technology found)
- `sumble-pp-cli organizations find` — Find organizations matching technology/category/firmographic filters (5 credits per row returned)
- `sumble-pp-cli organizations intelligence-brief` — AI-generated intelligence brief for an organization (50 credits when complete; 202 while pending is free)
- `sumble-pp-cli organizations match` — Resolve up to 1000 company names/URLs/locations to Sumble organizations (1 credit per matched org; unmatched free)

**people** — Find, traverse, and enrich people at organizations

- `sumble-pp-cli people enrich` — Reveal a person's email (10 credits) and/or phone (80 credits); cached or unavailable reveals are free
- `sumble-pp-cli people find` — Find people at an organization by job function/level/country (1 credit per person)
- `sumble-pp-cli people find-related-people` — Find people above/below a person in the org chart (1 credit per person)

**postings** — Find job postings and the people behind them — Sumble's hiring-signal layer

- `sumble-pp-cli postings find` — Find job postings by technology/category/country (2 credits per job, 3 with descriptions)
- `sumble-pp-cli postings find-related-people` — Find people associated with a job posting (1 credit per person)
- `sumble-pp-cli postings get` — Get a single job posting with its full description (1 credit)

**technologies** — Search Sumble's technology taxonomy

- `sumble-pp-cli technologies` — Search technologies by name; returns canonical slugs (1 credit only if at least one match, else free)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sumble-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Cost-safe ICP pull

```bash
sumble-pp-cli cost-estimate organizations.find --rows 50
```

Preview the spend before running the billed 'organizations find'; pair with 'budget set 300' to hard-cap it.

### Narrow a verbose org payload for an agent

```bash
sumble-pp-cli organizations find --filters-technologies '["databricks"]' --limit 50 --agent --select name,domain,account_score
```

organizations/find returns large rows; --select keeps only the fields the agent needs (the array is unwrapped, so paths are bare field names) and --agent emits compact structured output.

### Find buying-committee people

```bash
sumble-pp-cli people find --organization-domain stripe.com --filters-job-functions '["Engineer"]' --filters-job-levels '["VP","Director"]'
```

1 credit per person; cost-estimate people.find --rows N previews the spend first.

### Free CRM reconciliation before enrich

```bash
sumble-pp-cli reconcile accounts.csv --json
```

Resolve names to Sumble IDs cheaply (1 credit per match, unmatched free), then enrich only the accounts that matter.

### Offline competitive teardown

```bash
sumble-pp-cli stack-diff stripe.com adyen.com
```

Fetches and caches each org's tech stack once via enrich, then diffs shared vs unique technologies; re-runs are free from the cache.

## Auth Setup

Run `sumble-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
sumble-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `SUMBLE_API_KEY` as an environment variable.

Run `sumble-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sumble-pp-cli contact-lists list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
sumble-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sumble-pp-cli feedback --stdin < notes.txt
sumble-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.sumble-pp-cli/feedback.jsonl`. They are never POSTed unless `SUMBLE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUMBLE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sumble-pp-cli profile save briefing --json
sumble-pp-cli --profile briefing contact-lists list
sumble-pp-cli profile list --json
sumble-pp-cli profile show briefing
sumble-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sumble-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add sumble-pp-mcp -- sumble-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sumble-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sumble-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sumble-pp-cli <command> --help`.

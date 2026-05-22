---
name: pp-prospeo
description: "Every Prospeo endpoint, plus a credit-aware local cache, bulk CSV pipelines Trigger phrases: `enrich this lead with prospeo`, `find an email for this contact`, `find emails for this CSV`, `build a lookalike list`, `score these leads against my ICP`, `how many credits do I have left`, `use prospeo`, `run prospeo`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - prospeo-pp-cli
---

# Prospeo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `prospeo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install prospeo --cli-only
   ```
2. Verify: `prospeo-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps the full Prospeo API with an offline SQLite cache that mirrors lifetime-dupe semantics, a credit pre-flight guard, and search-then-enrich pipelines. Designed so a single agent can run a list-build → enrich → score → CSV flow with one command and never overspend.

## When to Use This CLI

Use prospeo-pp-cli whenever an agent needs to enrich contacts or companies, build a B2B lead list, or look up a person by LinkedIn URL. Prefer it over hand-rolled Python scripts because the local cache makes duplicate enrichments free, the credit pre-flight stops surprise overspend, and the find command answers 'do we already have this contact?' without an API call.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`find`** — Search every contact or company you've previously enriched without spending a credit.

  _When the agent already has a contact in cache, re-fetching from Prospeo burns credits. find is zero-cost._

  ```bash
  prospeo-pp-cli find 'VP of Sales fintech Berlin' --agent
  ```
- **`cache predict`** — Predict which rows in a CSV would be free Prospeo lifetime-duplicate hits before you spend credits.

  _Cuts bulk-run cost by skipping cold-API trips for guaranteed-free hits._

  ```bash
  prospeo-pp-cli cache predict --input leads.csv --agent
  ```
- **`score`** — Score every enriched row against a YAML ICP spec (titles, sizes, geos, tech, seniority) and emit a score column with reasons.

  _Lets the agent rank a list for the SDR without paying for a separate ICP-scoring vendor._

  ```bash
  prospeo-pp-cli score --icp icp.yaml --input enriched.csv --output scored.csv --json
  ```

### Credit economy controls
- **`person bulk --max-cost`** — Estimate the credit cost of a bulk job and refuse to run if it exceeds the budget or remaining balance.

  _Stops accidental 10-credit-per-row mobile bursts that drain the monthly allowance._

  ```bash
  prospeo-pp-cli person bulk --input leads.csv --max-cost 500 --mobile
  ```
- **`person bulk --dry-run`** — Show projected credits, dupe-hit ratio, and ETA before sending any rows.

  _Agents can decide whether a job is worth running based on cost projection._

  ```bash
  prospeo-pp-cli person bulk --input leads.csv --dry-run --json --select projected_cost,dupe_hits,api_calls
  ```
- **`credits burn`** — Daily burn rate and projected runway from snapshots of /account-information.

  _Tells the agent when the user is on track to run out before quota renewal._

  ```bash
  prospeo-pp-cli credits burn --days 30 --json
  ```
- **`person bulk --merge`** — Skip rows already in an existing enriched CSV so re-runs are incremental and cheap.

  _Re-running a campaign on a refreshed lead list only burns credits on net-new rows._

  ```bash
  prospeo-pp-cli person bulk --input new.csv --merge existing.csv --output merged.csv
  ```
- **`ledger`** — Report spend per CSV source, per campaign tag, or per day from the local enrichments + csv_jobs + account_snapshots ledger.

  _Answers 'which file ate my credits this week' in one command._

  ```bash
  prospeo-pp-cli ledger by-source --json --select source,credits_spent,rows_enriched
  ```

### Search composition
- **`lookalike`** — Find companies (or people) similar to a seed by auto-deriving industry, size, tech-stack, and location filters from an enriched seed.

  _Turns 'find more like this customer' into one command instead of three._

  ```bash
  prospeo-pp-cli lookalike --seed-company stripe.com --employees-min 200 --employees-max 1000 --agent
  ```
- **`person funnel`** — Run search-person with filters, walk pages, enrich each hit (verified-only), cache everything, write CSV.

  _Replaces 25+ HTTP calls and the glue between them with one invocation._

  ```bash
  prospeo-pp-cli person funnel --filters search.json --max 25 --output funnel.csv
  ```

### Reachability mitigation
- **`(internal — all commands)`** — Plan-aware token bucket that slows down before tripping Prospeo's 429s.

  _Avoids burning retries chasing the limit on a Starter or Growth plan._

  ```bash
  prospeo-pp-cli person bulk --input leads.csv --concurrency 5
  ```

## Command Reference

**account** — API key account information and credit balance

- `prospeo-pp-cli account` — Show current plan, remaining credits, used credits, next quota renewal, and team size. Free, does not consume credits.

**company** — Company enrichment and search across firmographics, tech stack, funding, location, hiring

- `prospeo-pp-cli company bulk` — Enrich up to 50 companies in one call. Auto-chunks CSV input into 50-row requests with caller IDs.
- `prospeo-pp-cli company enrich` — Enrich a company by website, LinkedIn URL, or Prospeo company_id.
- `prospeo-pp-cli company search` — Search companies by industry, size, revenue, funding, tech stack, location, hiring signals (Growth+ for ICP filters).

**person** — Person enrichment: find email, mobile, and full profile from name, linkedin URL, email, or person ID

- `prospeo-pp-cli person bulk` — Enrich up to 50 person records in one call.
- `prospeo-pp-cli person enrich` — Enrich a single person.
- `prospeo-pp-cli person search` — Search Prospeo's people database with filters (job_title, seniority, department, company, location, ICP signals).

**suggestions** — Search-helper autocomplete endpoints. Free, do not consume credits.

- `prospeo-pp-cli suggestions` — Autocomplete for location or job title filters before composing a search. 15 req/s rate limit.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
prospeo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `prospeo-pp-cli lookalike` — Build a search filter from a seed company or person and find similar contacts.
- `prospeo-pp-cli credits` — Show credit balance, daily burn, and projected runway based on recent enrichments.
- `prospeo-pp-cli cache` — Inspect and prune the local enrichment cache (lifetime-dupe hits are free; cache makes them instant).

## Recipes


### End-to-end SDR list build

```bash
prospeo-pp-cli person funnel --filters icp.json --max 100 --verified-only --output sdrs.csv
```

Searches Prospeo for people matching the ICP filter, walks pages until 100 hits, enriches each with a verified email, and writes a CSV ready for SmartLead.

### Predict bulk cost before spending

```bash
prospeo-pp-cli person bulk --input new-leads.csv --merge enriched-archive.csv --dry-run --json --select projected_cost,dupe_hits,api_calls,uncached_rows
```

Counts how many rows are guaranteed-free lifetime dupes vs. cold enrichments. Output JSON is narrowed to the four fields the agent needs.

### Score a list against ICP and export top hits

```bash
prospeo-pp-cli score --icp icp.yaml --input enriched.csv --output scored.csv --json --select rows.email,rows.score,rows.score_reasons
```

Reads enriched.csv, applies the YAML rule set, writes scored.csv with a score column, and emits a narrowed JSON view per row for the agent to rank against.

### Find similar customers to your best logo

```bash
prospeo-pp-cli lookalike --seed-company stripe.com --employees-min 200 --employees-max 2000 --json
```

Enriches the seed, derives industry + size + tech-stack filters, runs search-company, and returns the top 25 lookalikes ranked by similarity.

### Credit-spend forensics

```bash
prospeo-pp-cli ledger by-source --days 7 --json --select source,credits_spent,rows_enriched
```

Joins enrichments × csv_jobs × account_snapshots; answers 'which CSV burned my credits this week' in one command.

## Auth Setup

Set PROSPEO_API_KEY from https://app.prospeo.io/api-keys. The CLI sends it as the X-KEY header on every request. Run `doctor` to verify auth and show your remaining credits.

Run `prospeo-pp-cli doctor` to verify setup.

## Cache (Supabase) Setup

The cache features (`find`, `cache predict`, `ledger`, `credits burn`, `person funnel`) require Supabase with the `outreach` schema exposed. See README.md for the full step-by-step (env vars, PGRST_DB_SCHEMAS, service-role grants). Quick summary:

1. `export SUPABASE_URL=...` and `export SUPABASE_SERVICE_KEY=...`
2. Add `outreach` to `PGRST_DB_SCHEMAS` in your PostgREST config and restart it.
3. `prospeo-pp-cli outreach map` — one-time interactive wizard that probes your `outreach.people` / `outreach.companies` columns and writes `~/.config/prospeo-pp-cli/outreach-mapping.md`. Subsequent commands use this mapping to upsert enriched data into the right columns.

If Supabase is unset the CLI still runs; cache commands report the missing config. Live enrich/search/account commands always work.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  prospeo-pp-cli company search --agent --select id,name,status
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
prospeo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
prospeo-pp-cli feedback --stdin < notes.txt
prospeo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.prospeo-pp-cli/feedback.jsonl`. They are never POSTed unless `PROSPEO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PROSPEO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
prospeo-pp-cli profile save briefing --json
prospeo-pp-cli --profile briefing company search
prospeo-pp-cli profile list --json
prospeo-pp-cli profile show briefing
prospeo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `prospeo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add prospeo-pp-mcp -- prospeo-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which prospeo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   prospeo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `prospeo-pp-cli <command> --help`.

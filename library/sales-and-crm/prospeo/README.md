# Prospeo CLI

**Every Prospeo endpoint, plus a credit-aware local cache, bulk CSV pipelines, and lookalike search no other Prospeo tool ships.**

Wraps the full Prospeo API with an offline SQLite cache that mirrors lifetime-dupe semantics, a credit pre-flight guard, and search-then-enrich pipelines. Designed so a single agent can run a list-build → enrich → score → CSV flow with one command and never overspend.

Printed by [@riccardovandra](https://github.com/riccardovandra).

## Install

The recommended path installs both the `prospeo-pp-cli` binary and the `pp-prospeo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install prospeo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install prospeo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install prospeo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install prospeo --agent claude-code
npx -y @mvanhorn/printing-press install prospeo --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/prospeo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-prospeo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-prospeo --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-prospeo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-prospeo. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/prospeo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PROSPEO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "prospeo": {
      "command": "prospeo-pp-mcp",
      "env": {
        "PROSPEO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set PROSPEO_API_KEY from https://app.prospeo.io/api-keys. The CLI sends it as the X-KEY header on every request. Run `doctor` to verify auth and show your remaining credits.

## Supabase cache setup (optional, recommended)

`find`, `cache predict`, `ledger`, `credits burn`, and `person funnel` use a Supabase Postgres database as the lead/contact cache. The cache lives in the `outreach` schema (tables: `people`, `companies`).

If you do not configure Supabase the CLI still runs — the cache features will simply tell you "SUPABASE_URL and SUPABASE_SERVICE_KEY must be set." Live enrich, search, and account commands work without it.

To enable the cache:

1. **Export credentials:**
   ```bash
   export SUPABASE_URL=https://your-supabase-host
   export SUPABASE_SERVICE_KEY=<service-role-jwt>
   ```
2. **Expose the `outreach` schema in PostgREST.** PostgREST does not expose application schemas by default. In your Supabase deployment (Coolify, Docker, or hosted), set the `PGRST_DB_SCHEMAS` environment variable on the `postgrest` (a.k.a. `supabase-rest`) service to include `outreach`:
   ```
   PGRST_DB_SCHEMAS=public,outreach,storage,graphql_public
   ```
   No spaces around commas. Restart the postgrest container (and any kong proxy in front of it) so the new schema list takes effect.
3. **Grant access to the service role** (one-time, via psql):
   ```sql
   GRANT USAGE ON SCHEMA outreach TO service_role;
   GRANT ALL ON ALL TABLES IN SCHEMA outreach TO service_role;
   GRANT ALL ON ALL SEQUENCES IN SCHEMA outreach TO service_role;
   ```
4. **Verify:**
   ```bash
   prospeo-pp-cli doctor
   ```
   The `Supabase` row should read `reachable`. If it reports `not exposed`, re-check step 2.
5. **Generate the field mapping** so the CLI knows which `outreach.people` / `outreach.companies` columns receive which Prospeo response fields:
   ```bash
   prospeo-pp-cli outreach map
   ```
   The wizard probes your live schema, auto-matches columns that share names with Prospeo response fields (first_name, linkedin_url, current_job_title, email, etc.), and asks you only about fields where the auto-match is ambiguous. The result is written to `~/.config/prospeo-pp-cli/outreach-mapping.md` — a markdown doc with a YAML block you can hand-edit later. Override the location with `PROSPEO_MAPPING_PATH`.

   Subsequent commands read this mapping at startup. `outreach review` prints it back; `outreach reset` deletes it.

## Quick Start

```bash
# Verify your API key and show your current plan, credits, and quota renewal date
prospeo-pp-cli doctor


# Enrich a single person by LinkedIn URL (1 credit, free if you've enriched this person before)
prospeo-pp-cli person enrich --linkedin https://www.linkedin.com/in/williamhgates --json


# Project the credit cost and dupe-hit ratio of a bulk job before spending
prospeo-pp-cli person bulk --input leads.csv --output enriched.csv --dry-run


# Run the bulk enrich with a hard budget ceiling — the CLI aborts if the projected cost exceeds 500 credits
prospeo-pp-cli person bulk --input leads.csv --output enriched.csv --max-cost 500


# Search the local cache for anyone you've already enriched matching this query — zero credit cost
prospeo-pp-cli find 'VP of Sales fintech Berlin' --json

```

## Unique Features

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

## Usage

Run `prospeo-pp-cli --help` for the full command reference and flag list.

## Commands

### account

API key account information and credit balance

- **`prospeo-pp-cli account`** - Show current plan, remaining credits, used credits, next quota renewal, and team size. Free, does not consume credits.

### company

Company enrichment and search across firmographics, tech stack, funding, location, hiring

- **`prospeo-pp-cli company bulk`** - Enrich up to 50 companies in one call. Auto-chunks CSV input into 50-row requests with caller IDs.
- **`prospeo-pp-cli company enrich`** - Enrich a company by website, LinkedIn URL, or Prospeo company_id. 1 credit per match; misses and lifetime duplicates are free.
- **`prospeo-pp-cli company search`** - Search companies by industry, size, revenue, funding, tech stack, location, hiring signals (Growth+ for ICP filters). 25 per page, max 25,000 total. 1 credit per page with results.

### person

Person enrichment: find email, mobile, and full profile from name, linkedin URL, email, or person ID

- **`prospeo-pp-cli person bulk`** - Enrich up to 50 person records in one call. Pass --input CSV with first_name/last_name/company columns (or linkedin_url). Auto-chunks into 50-row requests.
- **`prospeo-pp-cli person enrich`** - Enrich a single person. Provide at least one matching path: linkedin_url, email, person_id, or first_name+last_name with a company hint. Costs 1 credit for email or 10 with --mobile.
- **`prospeo-pp-cli person search`** - Search Prospeo's people database with filters (job_title, seniority, department, company, location, ICP signals). Returns up to 25 hits per page; total cap 25,000. Costs 1 credit per page that returns results.

### suggestions

Search-helper autocomplete endpoints. Free, do not consume credits.

- **`prospeo-pp-cli suggestions`** - Autocomplete for location or job title filters before composing a search. 15 req/s rate limit. Pass exactly one of --location or --job-title.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
prospeo-pp-cli company search

# JSON for scripting and agents
prospeo-pp-cli company search --json

# Filter to specific fields
prospeo-pp-cli company search --json --select id,name,status

# Dry run — show the request without sending
prospeo-pp-cli company search --dry-run

# Agent mode — JSON + compact + no prompts in one flag
prospeo-pp-cli company search --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
prospeo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/prospeo-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PROSPEO_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `prospeo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PROSPEO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports 401 / 'invalid API key'** — Re-export PROSPEO_API_KEY from https://app.prospeo.io/api-keys and rerun `prospeo-pp-cli doctor`
- **429 / RATE_LIMITED on bulk enrich** — Lower --concurrency (Starter caps at 5 req/s; Growth 5; Pro 30). The built-in adaptive limiter slows down on `x-minute-request-left` headers.
- **INSUFFICIENT_CREDITS error mid-bulk** — Run `prospeo-pp-cli credits burn --days 14` to see your projected runway; use `--max-cost N` on bulk to fail fast before partial spend.
- **Mobile enrichment costs 10x more than expected** — The --mobile flag opts into the 10-credit mobile finder. Omit it for email-only (1 credit per row).

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

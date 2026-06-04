---
name: pp-botsee
description: "Have your agents measure and improve your brand's AI Visibility with botsee.io. API-first, agent-native. Trigger phrases: `check my AI visibility`, `audit my AI visibility`, `audit example.com`, `what AI is saying about my brand`, `show competitor mentions in ChatGPT`, `run botsee`, `use botsee`, `botsee audit`."
author: "grahac"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - botsee-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/botsee/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# BotSee — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `botsee-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install botsee --cli-only
   ```
2. Verify: `botsee-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

BotSee is the API-first GEO platform; this CLI is the API-first front door. The flagship `ai-visibility-audit <url>` mirrors the proven Python create-site+analyze flow as a single idempotent Go command. The other 64 commands cover the entire BotSee surface (CRUD on sites/customer-types/personas/questions, the full analysis lifecycle, recommendations, blog content, billing, signup including USDC/x402, webhooks, api-key rotation, usage) plus framework commands (sync/search/sql/doctor) over a local SQLite cache.

## When to Use This CLI

Reach for this CLI when an agent or pipeline needs structured BotSee data without burning credits on repeat reads. Best for: running a fresh AI-visibility audit (`ai-visibility-audit <url>` is the headline), inspecting an existing site's setup (`site-config`), generating recommendations from a completed analysis, or aggregating cited sources across multiple sites for a portfolio view. Not the right tool for hosted dashboard UI use — that's `app.botsee.io`.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Flagship workflow
- **`ai-visibility-audit`** — One command runs the full BotSee audit: idempotent on existing sites, bootstraps customer types and personas and questions if missing, then runs analysis across every LLM and prints a structured visibility report.

  _This is THE command. Reach for it when an agent or user says 'audit my AI visibility' or 'check how AI sees example.com' — it handles the full lifecycle._

  ```bash
  botsee-pp-cli ai-visibility-audit example.com --types 2 --personas 2 --questions 5 --watch --agent
  ```
- **`recommendations`** — Generate next-step recommendations from an analysis — promoted to top-level so agents can discover it without descending into the analysis subcommand tree.

  _Use after an analysis to get LLM-generated action items without re-spending if you already pulled them once._

  ```bash
  botsee-pp-cli recommendations $ANALYSIS_UUID --agent
  ```

### Workflow plumbing
- **`site-config`** — Print the full customer-types -> personas -> questions tree for a site, with UUIDs and the actual edit commands users can copy-paste to add, update, or remove any node.

  _Reach for this when a user asks 'what is set up for this site' or 'show me my BotSee config' — it surfaces every UUID needed for follow-up edits._

  ```bash
  botsee-pp-cli site-config --site $SITE_UUID --agent
  ```
- **`sites-summary`** — Aggregate cited sources across every synced site, grouped by domain, with citation count, distinct sites citing each domain, and first-seen timestamp.

  _Use for multi-site users and agencies who need the cross-portfolio answer 'which sources keep getting cited everywhere' — impossible without local aggregation._

  ```bash
  botsee-pp-cli sites-summary --agent --select domain,citation_count,distinct_sites_citing,first_seen
  ```

## Command Reference

**account** — Manage account

- `botsee-pp-cli account` — Returns account details including company name, site count, and owner information.

**analysis** — Manage analysis

- `botsee-pp-cli analysis get` — Returns analysis details including status. Poll this endpoint until status is 'completed' or 'failed'.
- `botsee-pp-cli analysis run` — Starts an analysis run. This is asynchronous - poll GET /api/v1/analysis/:uuid for status. Returns 202 Accepted.

**api-keys** — Manage api keys

- `botsee-pp-cli api-keys create` — Creates a new API key. The raw key is returned only once in the response body — store it immediately.
- `botsee-pp-cli api-keys delete` — Revokes an API key. You cannot revoke the key being used to make this request — use rotate or another key.
- `botsee-pp-cli api-keys list` — Lists all API keys for the organization (raw key never returned).
- `botsee-pp-cli api-keys reset` — Exchanges a one-time reset token (from email) for a fresh API key.

**billing** — Manage billing

- `botsee-pp-cli billing get-settings` — Returns the organization's billing settings and current credit balance.
- `botsee-pp-cli billing topoff-via-x402` — Discovery call without payment headers returns 402 and does not require auth.
- `botsee-pp-cli billing update-settings` — Updates the organization's monthly spend limit. Other settings are read-only via this endpoint.

**botsee-auth** — Manage botsee auth

- `botsee-pp-cli botsee-auth` — Validates the API key and returns organization info and credit balance.

**customer-types** — Manage customer types

- `botsee-pp-cli customer-types delete` — Archives a customer type. Returns 204 No Content on success.
- `botsee-pp-cli customer-types get` — Returns a customer type with its personas.
- `botsee-pp-cli customer-types update` — Updates a customer type. Only include fields you want to change.

**personas** — Manage personas

- `botsee-pp-cli personas delete` — Archives a persona. Returns 204 No Content on success.
- `botsee-pp-cli personas get` — Returns a persona with its questions.
- `botsee-pp-cli personas update` — Updates a persona. Only include fields you want to change.

**pricing** — Manage pricing

- `botsee-pp-cli pricing` — Returns the credit cost for each chargeable operation.

**questions** — Manage questions

- `botsee-pp-cli questions delete` — Deletes a question. Returns 204 No Content on success.
- `botsee-pp-cli questions update` — Updates a question. Only include fields you want to change.

**rate-limits** — Manage rate limits

- `botsee-pp-cli rate-limits` — Returns the caller's current rate-limit state without consuming additional budget.

**signup** — Manage signup

- `botsee-pp-cli signup via-cc` — Creates a credit-card signup token. USDC signups must use `/api/v1/signup/usdc`.
- `botsee-pp-cli signup via-usdc-token` — Creates a USDC signup token. Use `no_email: true` for autonomous agent flows (no setup_url returned).

**sites** — Manage sites

- `botsee-pp-cli sites create` — Creates a new site. Auto-generates product_name and value_proposition from URL if not provided (5 credits).
- `botsee-pp-cli sites delete` — Archives a site. Returns 204 No Content on success.
- `botsee-pp-cli sites get` — Returns a site with its customer types and persona counts.
- `botsee-pp-cli sites list` — Returns a paginated list of sites for the organization.

**usage** — Manage usage

- `botsee-pp-cli usage by-key` — Returns credit usage breakdown per API key.
- `botsee-pp-cli usage get` — Returns credit balance, auto-charge settings, and paginated transaction history.

**webhooks** — Manage webhooks

- `botsee-pp-cli webhooks create` — Registers a webhook URL. Returns the webhook with its signing secret (shown only once).
- `botsee-pp-cli webhooks delete` — Deletes a webhook. Returns 204 No Content on success.
- `botsee-pp-cli webhooks list` — Lists all registered webhooks for the organization.
- `botsee-pp-cli webhooks list-events` — Returns the catalog of event types this API can emit, with JSON Schemas per event.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
botsee-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Audit a new domain end-to-end

```bash
botsee-pp-cli ai-visibility-audit example.com --watch --output report.md
```

Creates the site, generates structure, runs analysis, writes a markdown report. Re-running on the same domain just runs a fresh analysis (no duplicate site).

### Preview the cost before running

```bash
botsee-pp-cli ai-visibility-audit example.com --estimate-only --agent
```

Returns predicted credit + USD cost without spending — reads the live `/pricing` endpoint for current multipliers.

### Inspect what's configured for a site

```bash
botsee-pp-cli site-config --site $SITE_UUID --agent --select customer_types,personas,questions
```

Returns the nested tree as JSON — agents can walk it to find UUIDs for follow-up `personas update / questions delete` calls.

### Find sources getting cited across every brand

```bash
botsee-pp-cli sites-summary --agent --select domain,citation_count,distinct_sites_citing
```

Cross-site domain authority rollup — impossible via the BotSee API which is per-site only.

### Search every LLM response for a competitor mention

```bash
botsee-pp-cli search "competitor name" --type responses --limit 20 --agent --select analysis_uuid,model,response_text
```

FTS5 over every raw LLM response synced locally — narrows the deeply-nested response payload to just the fields agents need.

## Auth Setup

BotSee uses Bearer tokens prefixed `bts_live_`. Set `BOTSEE_API_KEY` in your environment and the CLI threads it into every authenticated call. Manage keys with `api-keys list / create / rotate / reset / delete` — `reset` consumes a one-time token emailed to the account holder if the key was lost. The CLI never logs key values.

Run `botsee-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  botsee-pp-cli account --agent --select id,name,status
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
botsee-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
botsee-pp-cli feedback --stdin < notes.txt
botsee-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.botsee-pp-cli/feedback.jsonl`. They are never POSTed unless `BOTSEE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BOTSEE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
botsee-pp-cli profile save briefing --json
botsee-pp-cli --profile briefing account
botsee-pp-cli profile list --json
botsee-pp-cli profile show briefing
botsee-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `botsee-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add botsee-pp-mcp -- botsee-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which botsee-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   botsee-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `botsee-pp-cli <command> --help`.

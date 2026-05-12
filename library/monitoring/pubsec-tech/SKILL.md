---
name: pp-pubsec-tech
description: "Federal IT spending, opportunities, and news joined into one local store — searchable, scriptable, and agent-ready. Trigger phrases: `federal IT contracts`, `federal IT spending`, `find IT opportunities`, `what did GSA spend on`, `recompete radar`, `vendor rollup`, `explain this federal news article`, `use pubsec-tech`, `run pubsec-tech`."
author: "Abe Rosloff"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pubsec-tech-pp-cli
---

# pubsec-tech — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `pubsec-tech-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install pubsec-tech --cli-only
   ```
2. Verify: `pubsec-tech-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/cmd/pubsec-tech-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

pubsec-tech wraps USAspending.gov (federal awards), SAM.gov Opportunities (live RFPs), and federal-tech RSS feeds (Nextgov/FCW, FedScoop, CyberScoop, MeriTalk, GovExec Technology, Federal News Network, and more) behind a single Go CLI. Unique to this CLI: cross-source joins that the upstream APIs never expose together — vendor rollups, recompete radar, news-to-contract correlation, and anti-hallucination NAICS/PSC guards.

## When to Use This CLI

Use pubsec-tech when you need to query federal IT spending, find live IT contract opportunities, or correlate federal-tech news with the underlying contracts. It is the right choice for any task that joins two or more of: USAspending awards, SAM.gov opportunities, and federal-tech news (Nextgov, FedScoop, CyberScoop, MeriTalk, GovExec, FNN). It is not the right choice for grants-only research (use Grants.gov directly), defense-specific operational queries (use Defense News + service-specific APIs), or state/local contract data outside the optional StateScoop/Route Fifty RSS sources.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`vendor`** — Joins SAM registration, exclusions, USAspending recipient profile, open opportunities, and recent news mentions for one federal vendor (needs DATA_GOV_API_KEY for the SAM.gov tier; pre-sync awards/recipients/news for full coverage).

  _When the user asks about one company in federal IT, this is the one-shot answer — saves a dozen API calls and the join logic._

  ```bash
  pubsec-tech-pp-cli vendor "Leidos" --agent
  ```
- **`recompete`** — Lists federal IT awards whose period of performance ends inside a chosen window, joined to the incumbent's profile and to any follow-on RFP already posted on SAM (run `sync --resources awards` first).

  _Lets a BD analyst or reporter find contracts that are about to recompete before they show up as news._

  ```bash
  pubsec-tech-pp-cli recompete --naics 541512 --within 18m --agent
  ```
- **`news link`** — Fetches RSS articles and links each one to the underlying contracts and opportunities by deterministic name-match against local recipient and agency tables (needs `news sync` plus `sync --resources recipients,agencies` for entity matches).

  _Turns a federal-tech news feed into a structured set of references to actual contracts and RFPs._

  ```bash
  pubsec-tech-pp-cli news link --since 7d --agent
  ```
- **`agency`** — For one agency, returns open IT opportunities, recent IT awards, IT-NAICS spend trend, and recent news mentions in one composed view.

  _Answers 'what's happening in agency X on the IT side' in one call instead of three searches across three websites._

  ```bash
  pubsec-tech-pp-cli agency DOD --modernization --agent
  ```
- **`watch vendor`** — Persistent watchlist that returns the news articles touching a vendor since the last recorded watch-tick (use --peek to read without advancing).

  _Reporters and analysts watching dozens of vendors get only the article deltas they care about._

  ```bash
  pubsec-tech-pp-cli watch vendor "Leidos" --agent
  ```

### Agent-native plumbing
- **`digest`** — One structured bundle: new awards in window, upcoming recompetes, open-opportunity hits, and recent news, scoped to a comma-separated NAICS list or a saved profile.

  _Drops a 2.5-hour Monday-morning BD ritual to a single command._

  ```bash
  pubsec-tech-pp-cli digest --since 7d --naics 541511,541512,541519 --agent
  ```
- **`explain`** — Given a news article URL or headline string, returns the article's text plus the linked recipients and opportunities, with the matched mention spans.

  _A reporter or analyst pasting a URL gets the contract context, not just the article text._

  ```bash
  pubsec-tech-pp-cli explain "https://www.nextgov.com/..."
  ```
- **`code resolve`** — Resolves a plain term ('cloud', 'cybersecurity') to a NAICS or PSC code only if it matches a real row in the local reference tables; on miss, returns suggestions and exits non-zero rather than guessing.

  _Agents hallucinate NAICS codes constantly; this turns a silent empty-result-set into an actionable error._

  ```bash
  pubsec-tech-pp-cli code resolve "cloud" --kind naics --agent
  ```

### Reachability mitigation
- **`opps eligible`** — Filters open opportunities to those whose set-aside categories match the SAM-registered socioeconomic indicators of a given UEI (needs DATA_GOV_API_KEY plus `sync --resources entities,opportunities`).

  _Vendors only see opportunities they can actually bid on; saves time triaging._

  ```bash
  pubsec-tech-pp-cli opps eligible ABC1234567 --agent
  ```

## Command Reference

**agencies** — Federal agencies (toptier and subtier) - USAspending

- `pubsec-tech-pp-cli agencies budget_function` — Agency obligations grouped by budget function (e.g. National Defense, General Science) for one fiscal year
- `pubsec-tech-pp-cli agencies get` — Get one agency's profile by toptier code (e.g. 097 for DoD, 047 for GSA, 014 for Treasury)
- `pubsec-tech-pp-cli agencies list` — List every toptier federal agency with budget, obligations, and outlays for the current fiscal year. Use this to...
- `pubsec-tech-pp-cli agencies obligations_by_award_category` — Agency obligations split by award category (contracts, grants, direct payments, loans, other)
- `pubsec-tech-pp-cli agencies subagencies` — List subagencies (bureaus and offices) under a toptier agency, with obligation totals

**awards** — Federal contract and assistance awards - USAspending

- `pubsec-tech-pp-cli awards get` — Get full detail for one award by generated_internal_id (the CONT_AWD_... or ASST_NON_... identifier)
- `pubsec-tech-pp-cli awards search` — Search awards by filter combination (NAICS, PSC, time, agency, dollar bounds, recipient). Pass the filters object as...
- `pubsec-tech-pp-cli awards subawards` — List subawards under a prime award

**entities** — SAM.gov registered entities, exclusions, and 889 compliance - SAM.gov

- `pubsec-tech-pp-cli entities` — Search SAM-registered entities by UEI, CAGE, DUNS, or legal business name. Requires DATA_GOV_API_KEY.

**opportunities** — Live federal contract opportunities (RFPs, sources sought, awards) - SAM.gov

- `pubsec-tech-pp-cli opportunities description` — Full RFP description body for one opportunity (separate v1 endpoint). Requires DATA_GOV_API_KEY.
- `pubsec-tech-pp-cli opportunities get` — Fetch a single SAM opportunity by notice ID. Requires DATA_GOV_API_KEY.
- `pubsec-tech-pp-cli opportunities search` — Search SAM.gov opportunities by NAICS, PSC, set-aside, agency, and posted-date window. Requires DATA_GOV_API_KEY...

**recipients** — Federal award recipients (vendors and grantees) - USAspending

- `pubsec-tech-pp-cli recipients autocomplete` — Server-side recipient autocomplete for resolving vendor names to UEIs
- `pubsec-tech-pp-cli recipients get` — Get a recipient profile including DUNS, UEI, alternate names, parent recipient, and total federal $
- `pubsec-tech-pp-cli recipients list` — List recipients with rollup totals; supports search by name or UEI

**references** — Reference data: NAICS, PSC, glossary, codes

- `pubsec-tech-pp-cli references glossary` — 151-term federal-spending glossary; pass term= to filter
- `pubsec-tech-pp-cli references naics` — NAICS code hierarchy lookup; pass naics= to filter to a code prefix

**spending** — Aggregated federal spending breakdowns - USAspending

- `pubsec-tech-pp-cli spending by_category` — Spending grouped by a named category (recipient, awarding_agency, awarding_subagency, federal_account, psc, cfda,...
- `pubsec-tech-pp-cli spending by_geography` — Spending aggregation by state, county, or congressional district
- `pubsec-tech-pp-cli spending over_time` — Time-series spending aggregation grouped by fiscal year, quarter, or month for a filter set


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
pubsec-tech-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

> **Sync prerequisite:** Cross-source commands (`vendor`, `news link`, `explain`, `digest`, `recompete`, `agency`, `watch vendor`) depend on the local SQLite store. Run `pubsec-tech-pp-cli sync` to populate USAspending awards/recipients/agencies and `pubsec-tech-pp-cli news sync` to pull RSS articles before invoking them — unsynced calls return empty results with an actionable note rather than errors.

### Confirm a NAICS code before any downstream query

```bash
pubsec-tech-pp-cli code resolve "computer systems design" --kind naics --agent
```

Anti-hallucination guard: returns the canonical NAICS code (541512) or exits non-zero with suggestions if the term doesn't match a real row. Always run this before querying USAspending with a NAICS filter.

### Daily digest scoped to your NAICS list

```bash
pubsec-tech-pp-cli digest --since 24h --naics 541511,541512,541519 --agent --select new_articles.title,upcoming_recompetes.recipient_name,opp_hit_count
```

Composes recompete + news-correlation against the IT NAICS set in one structured response. Demonstrates `--select` with dotted paths to narrow the verbose digest payload to only the fields an agent needs.

### Find recompete opportunities for IT services

```bash
pubsec-tech-pp-cli recompete --naics 541511,541512,541519 --within 18m --min-ceiling 50000000 --agent
```

Joins `awards.period_of_performance_current_end_date` with the incumbent recipient and any already-posted follow-on RFP. Requires `sync --resources awards,recipients` to have populated the store.

### Explain a news article

```bash
pubsec-tech-pp-cli explain "https://fedscoop.com/..." --agent
```

Returns the article plus the awards and opportunities it references, with the matched mention spans. Requires `news sync` plus `sync --resources recipients,agencies` so the mention extractor has entities to match against.

### Triage open opportunities for a small business

```bash
pubsec-tech-pp-cli opps eligible ABC1234567 --posted-from 14d --agent
```

Filters open SAM opportunities to those whose set-aside categories match the entity's SAM-registered socioeconomic indicators (8(a), SDVOSB, WOSB, HUBZone). Requires `DATA_GOV_API_KEY` in the environment and `sync --resources entities,opportunities` populated.

## Auth Setup

USAspending.gov and every RSS feed are free and keyless — the CLI works out of the box for `awards`, `recipients`, `agencies`, `spending`, `references`, `news`, and `code resolve`.

SAM.gov endpoints (`opportunities`, `entities`, `opps eligible`) require a free data.gov API key. Get one at https://api.data.gov/signup/, then:

```bash
export DATA_GOV_API_KEY=<your-key>
```

There is no `auth` subcommand — credentials are environment-only. Run `pubsec-tech-pp-cli doctor` to verify auth + connectivity.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  pubsec-tech-pp-cli agencies list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
pubsec-tech-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
pubsec-tech-pp-cli feedback --stdin < notes.txt
pubsec-tech-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.pubsec-tech-pp-cli/feedback.jsonl`. They are never POSTed unless `PUBSEC_TECH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PUBSEC_TECH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
pubsec-tech-pp-cli profile save briefing --json
pubsec-tech-pp-cli --profile briefing agencies list
pubsec-tech-pp-cli profile list --json
pubsec-tech-pp-cli profile show briefing
pubsec-tech-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `pubsec-tech-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add pubsec-tech-pp-mcp -- pubsec-tech-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which pubsec-tech-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   pubsec-tech-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `pubsec-tech-pp-cli <command> --help`.

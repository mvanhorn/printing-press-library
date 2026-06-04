---
name: pp-meta-ads
description: "The first agent-native Meta Ads CLI with local SQLite history and compound queries the live API cannot answer. Trigger phrases: `find creative fatigue on Meta`, `check Facebook ad performance`, `which Meta ads should I retire`, `Meta audience overlap`, `stuck in learning`, `use meta-ads`, `run meta-ads`."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - meta-ads-pp-cli
    install:
      - kind: go
        bins: [meta-ads-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/cmd/meta-ads-pp-cli
---

# Meta Ads — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `meta-ads-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install meta-ads --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `meta-ads-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/cmd/meta-ads-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you need creative-fatigue diagnostics, audience overlap cartography, learning-phase forensics, or attribution drift detection across Meta ad accounts. Best when paired with a daily sync into the local store so trend queries are instant. Read-only by design — never used to modify campaigns, budgets, or audiences.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to create, pause, modify, or delete campaigns, adsets, ads, audiences, or creatives — it is read-only.
- Do not use this CLI for budget management or bid-strategy changes — that is what Meta Ads Manager or facebook-business-sdk are for.
- Do not use this CLI for write-side audience uploads — custom audience creation from customer files is out of scope.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`fatigue`** — Detect ads whose CPM is drifting up, frequency is climbing, and CTR is falling across a configurable window.

  _Use this to decide which ads to retire. Spending on a fatigued creative is the highest-leverage waste in any account._

  ```bash
  meta-ads-pp-cli fatigue --campaign 23847265 --window 14d --agent
  ```
- **`decay`** — Compare a creative's first-impression CTR against its current CTR, with projected dead-date.

  _Use for retire/refresh decisions on a single creative. The slope and projected dead-date are the deciding factor._

  ```bash
  meta-ads-pp-cli decay --creative-id 120208734567 --agent
  ```
- **`overlap`** — Pairwise overlap percentages across custom audiences, with cannibalization recommendations.

  _Use when ad strategy is mature and ROAS feels diluted across similar audiences. Surfaces consolidate-or-exclude decisions._

  ```bash
  meta-ads-pp-cli overlap --audience 23847001 --audience 23847002 --audience 23847003 --agent
  ```
- **`learning`** — Surface adsets stuck in algorithmic learning >N days with root-cause hint (budget too low, audience too narrow, events too sparse).

  _Use when ROAS collapses across multiple adsets simultaneously. Stuck-in-learning adsets eat budget without optimizing._

  ```bash
  meta-ads-pp-cli learning --account act_4327210487520472 --min-days 7 --agent
  ```

### Forensic queries
- **`reconcile`** — Per-day diff between Meta-reported account spend and sum-of-insights spend; flags days where attribution drift exceeds threshold.

  _Use monthly for attribution audits. Surfaces specific days where Meta and insights disagree, usually delayed conversion attribution._

  ```bash
  meta-ads-pp-cli reconcile --account act_4327210487520472 --since 30d --threshold 5 --agent
  ```
- **`bottleneck`** — Highest-spend adsets with worst ROAS, ranked with 'why' column joining learning state and effective status.

  _Use weekly to decide what to pause. Highest-leverage targets surface first._

  ```bash
  meta-ads-pp-cli bottleneck --account act_4327210487520472 --limit 10 --agent
  ```

### Account hygiene
- **`stale`** — Active ads with zero impressions in N days.

  _Use quarterly for account hygiene. Active ads with zero impressions are usually misconfigured or post-deletion zombies._

  ```bash
  meta-ads-pp-cli stale --days 90 --agent
  ```
- **`inventory`** — Group every ad in an account by effective_status, surface ads where configured status='ACTIVE' but effective_status='WITH_ISSUES' or 'DISAPPROVED'.

  _Use first thing every morning. DISAPPROVED-but-still-ACTIVE ads cost real spend without delivering._

  ```bash
  meta-ads-pp-cli inventory --account act_4327210487520472 --by effective_status --agent
  ```

## Command Reference

**adcreatives** — Manage adcreatives

- `meta-ads-pp-cli adcreatives <adId>` — Get creatives attached to an ad

**ads** — Ad-level read operations

- `meta-ads-pp-cli ads <adAccountId>` — List ads in an ad account

**adsets** — Ad set-level read operations

- `meta-ads-pp-cli adsets <adAccountId>` — Returns ad sets with targeting summary, budget, optimization goal, and bid strategy.

**campaigns** — Campaign-level read operations

- `meta-ads-pp-cli campaigns <adAccountId>` — Returns campaigns with status, objective, and budget metadata.

**customaudiences** — Manage customaudiences

- `meta-ads-pp-cli customaudiences <adAccountId>` — List custom audiences in an ad account

**delivery-estimate** — Manage delivery estimate

- `meta-ads-pp-cli delivery-estimate <adAccountId>` — Get reach/impressions delivery estimate

**insights** — Performance and delivery insights (the creative-fatigue observatory)

- `meta-ads-pp-cli insights get-account` — Returns aggregated insights across the entire ad account.
- `meta-ads-pp-cli insights get-ad` — Ad-level insights with frequency, CPM, CTR over time.
- `meta-ads-pp-cli insights get-ad-set` — Get ad set-level insights
- `meta-ads-pp-cli insights get-campaign` — Get campaign-level insights

**me** — Manage me

- `meta-ads-pp-cli me` — Returns every ad account the access token has been granted access to.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
meta-ads-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Triage creative fatigue across a campaign

```bash
meta-ads-pp-cli fatigue --campaign 23847265 --window 14d --agent --select ad_id,ad_name,cpm_slope,frequency_now,ctr_slope,verdict
```

Returns one row per ad with CPM slope, current frequency, CTR slope, and a retire/keep verdict. Narrow output by selecting only the fatigue-relevant fields to keep agent context bounded.

### Find audience cannibalization risk

```bash
meta-ads-pp-cli overlap --audience 23847001 --audience 23847002 --audience 23847003 --agent
```

Returns pairwise overlap percentages and a recommendation per pair. Anything above 30% is a candidate for consolidation or exclusion.

### Spot stuck-in-learning adsets

```bash
meta-ads-pp-cli learning --account act_4327210487520472 --min-days 7 --agent
```

Surfaces adsets where learning_stage_info has been LEARNING for >7 days. The 'why' column hints at budget/audience/event sparsity.

### Daily inventory roll-up first thing

```bash
meta-ads-pp-cli inventory --account act_4327210487520472 --by effective_status --agent
```

Groups every ad by effective_status. The DISAPPROVED-but-configured-ACTIVE class is the highest-leverage daily finding.

### Monthly spend reconciliation

```bash
meta-ads-pp-cli reconcile --account act_4327210487520472 --since 30d --threshold 5 --agent
```

Per-day diff between Meta-reported account spend and sum-of-insights spend over the last 30 days. Flags days where drift exceeds 5%.

## Auth Setup

Authenticate with a Meta access token in the META_ACCESS_TOKEN env var. Generate from Graph API Explorer (User Access Token, 1-2h lifespan) for ad-hoc use, or Business Manager System Users (no expiry) for repeat use. Grant ads_read scope only — never ads_management or any write permission. Token discovery via /me/adaccounts lists every account accessible to the token; no account ID configuration required.

Run `meta-ads-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  meta-ads-pp-cli ads mock-value --agent --select id,name,status
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
meta-ads-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
meta-ads-pp-cli feedback --stdin < notes.txt
meta-ads-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/meta-ads-pp-cli/feedback.jsonl`. They are never POSTed unless `META_ADS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `META_ADS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
meta-ads-pp-cli profile save briefing --json
meta-ads-pp-cli --profile briefing ads mock-value
meta-ads-pp-cli profile list --json
meta-ads-pp-cli profile show briefing
meta-ads-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `meta-ads-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/cmd/meta-ads-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add meta-ads-pp-mcp -- meta-ads-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which meta-ads-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   meta-ads-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `meta-ads-pp-cli <command> --help`.

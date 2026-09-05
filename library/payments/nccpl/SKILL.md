---
name: pp-nccpl
description: "Pakistan's clearing-layer data as a local research panel with a coverage audit and an arithmetic self-check, not another dashboard. Trigger phrases: `foreign investor flows on the PSX`, `FIPI and LIPI for last week`, `which sectors did foreigners buy`, `short interest on a Pakistani stock`, `free float for a PSX symbol`, `MTS open positions`, `use nccpl`, `run nccpl`."
author: "qazmataz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - nccpl-pp-cli
    install:
      - kind: go
        bins: [nccpl-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/nccpl/cmd/nccpl-pp-cli
---

# NCCPL — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `nccpl-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install nccpl --cli-only
   ```
2. Verify: `nccpl-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/nccpl/cmd/nccpl-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

NCCPL publishes the only per-symbol leverage, short-interest and free-float data in the Pakistani market - once, behind Cloudflare, one date at a time, with no export. Every tool built on it renders a chart and throws the history away. This one keeps it: sync backfills a local SQLite panel, panel emits it in the shape a regression consumes, coverage tells you exactly which sessions are missing, and verify proves each date against NCCPL's own arithmetic identities before you trust it.

## When to Use This CLI

Reach for this CLI when you need Pakistani market data that sits below the exchange feed: who bought and sold by investor class, how much leverage and short interest is open against a symbol, what a symbol's free float and VAR margin are, and how trade volume reconciles against settlement. It is built for assembling history into a local panel and handing that panel to a model, so prefer it whenever the question spans more than one date. It is also the right tool for auditing data you already pulled, because it can prove a date against NCCPL's own arithmetic identities and tell you which sessions are missing.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for live or intraday prices, quotes, or index levels - NCCPL publishes post-settlement data once per session, not a market feed.
- Do not use this CLI for company fundamentals, earnings, announcements or corporate actions; it is a clearing house, not a filings source.
- Do not use this CLI to place, modify or settle trades - every endpoint it reaches is read-only market information.
- Do not use this CLI to compute a regression or backtest result; export a panel and run the analysis in your own research code.
- Do not expect deep history on every resource - some NCCPL surfaces are near-dormant, so run coverage before assuming an archive exists.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Research-grade integrity
- **`verify`** — Prove a date's flow numbers are internally consistent before they reach a regression.

  _Reach for this before feeding any NCCPL date into a model; a date that fails an invariant is corrupt input, not a weak signal._

  ```bash
  nccpl-pp-cli verify --from 2026-08-01 --to 2026-09-04 --agent
  ```
- **`coverage`** — List which sessions are missing per resource, how stale each one is, and how wide each date's data actually is.

  _Use this as a pipeline pre-flight - it exits non-zero on a gap, so a scheduled research job fails loudly instead of silently treating a missing session as a zero. Re-syncing a date replaces it rather than adding to it: a stored settlement date always mirrors the last snapshot fetched for it, so if NCCPL revises a date and drops symbols the next sync removes them and the store never serves a row the source stopped publishing. coverage row_count therefore always equals the number of observations actually stored, and the audit can never disagree with the data it audits. One deliberate exception: a fetch returning zero rows for a date that already holds rows does NOT delete them, because an empty response and a transient failure look identical from here; the next non-empty fetch mirrors the date correctly._

  ```bash
  nccpl-pp-cli coverage --resources fipi,lipi,var-margins --exit-code --agent
  ```
- **`contract-check`** — Assert every endpoint family still answers correctly for a date it just reported as its own latest.

  _Run this when results look empty - it separates an expired session or a changed request contract from a genuine no-data day._

  ```bash
  nccpl-pp-cli contract-check --agent
  ```

### Local history that compounds
- **`panel`** — Emit any synced resource as a tidy long-format panel, with gaps marked rather than filled.

  _This is the handoff from CLI to research store - it is the shape a regression consumes, and the observed_at column is what makes a flow number admissible as an ex-ante input._

  ```bash
  nccpl-pp-cli panel --resource fipi --from 2015-12-09 --to 2026-09-04 --agent
  ```
- **`universe`** — Reconstruct which symbols were listed and clearing-eligible on any past date.

  _Use this to state a screen's universe width in genuinely live names, and as a second opinion that can disagree with a price-staleness filter for reasons that filter cannot move for._

  ```bash
  nccpl-pp-cli universe --on 2019-03-15 --agent
  ```
- **`risk-changes`** — Date every step change in a symbol's free float, VAR margin and haircut.

  _Free float is the input a cap-weighted cross-section needs and no other public Pakistani source publishes it daily._

  ```bash
  nccpl-pp-cli risk-changes --since 30d --field free_float --agent
  ```

### Cross-market joins
- **`leverage`** — Join MTS, MFS and MSF open positions with SLB net open position into one per-symbol cross-section.

  _SLB net open position is the closest thing this market has to published short interest; pair it with MTS and MFS open interest to see leverage building in a name before a forced release._

  ```bash
  nccpl-pp-cli leverage --from 2026-08-01 --to 2026-09-04 --agent
  ```

### Reachability mitigation
- **`flows`** — Fetch daily FIPI/LIPI sector flows into the local store without a browser.

  _This is the one NCCPL dataset obtainable unattended, so it is what a scheduled daily job can actually collect._

  ```bash
  nccpl-pp-cli flows --from 2026-08-01 --to 2026-09-04 --agent
  ```
- **`ingest`** — Load NCCPL responses captured through your own browser into the same local store.

  _Reach for this when a dataset is gated: export a HAR from a normal browsing session and every other command works on it unchanged. Bodies can also be piped in directly with --stdin (pbpaste | nccpl-pp-cli ingest --stdin --resource var-margins --date 2026-09-04), so a single copied response never needs a temp file. Nothing unidentifiable is stored: a HAR entry is only ingested when it was served by nccpl.com.pk or a subdomain over https, and a response body must carry either the documented envelope key for the resource or exactly one of the API's own other envelope names. An all-traffic capture routinely holds other origins -- a proxy, a mock, a local dev server on the same path -- and those are refused and listed under skipped with a reason, never filed under a guess._

  ```bash
  nccpl-pp-cli ingest capture.har --agent
  ```
- **`capture`** — Fetch the gated NCCPL datasets through a controlled browser and store them locally; opt in with --launch, and add --headless to run with no window.

  _This is the only route to per-symbol free float, VAR margins, leverage positions and settlement data; every other command then reads them from the local store with no browser involved. With --headless no window appears and the run is unattended, so a scheduled job can keep the gated datasets current; Chrome must be installed because the clearance cannot be replayed by any non-browser HTTP client._

  ```bash
  nccpl-pp-cli capture --resources var-margins --latest-only --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 51 API entries from 204 total network entries
- Protocols: rest_json (75% confidence), html_scrape (55% confidence)
- Generation hints: browser_http_transport, requires_protected_client
- Candidate command ideas: create_data_by_date_range — Derived from observed POST /api/graph-data/data-by-date-range traffic.; create_rum — Derived from observed POST /cdn-cgi/rum traffic.; list_latest_data — Derived from observed GET /api/graph-data/latest-data traffic.; list_latest_date — Derived from observed GET /api/financiers-financees/latest-date traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**fipi** — Foreign Investors Portfolio Investment (FIPI) net flows by investor class and market segment.

- `nccpl-pp-cli fipi data` — Foreign Investors Portfolio Investment (FIPI) net flows by investor class and market segment. Date range.
- `nccpl-pp-cli fipi latest-date` — Most recent publication date available for fipi. Needs Cloudflare clearance only; no CSRF or session.

**fipi-normal** — FIPI buy/sell volume and value by client type and market type for one date.

- `nccpl-pp-cli fipi-normal data` — FIPI buy/sell volume and value by client type and market type for one date. Single settlement date.
- `nccpl-pp-cli fipi-normal latest-date` — Most recent publication date available for fipi-normal. Needs Cloudflare clearance only; no CSRF or session.

**fipi-sector** — FIPI net flows broken out by market sector.

- `nccpl-pp-cli fipi-sector data` — FIPI net flows broken out by market sector. Date range.
- `nccpl-pp-cli fipi-sector latest-date` — Most recent publication date available for fipi-sector. Needs Cloudflare clearance only; no CSRF or session.

**lipi** — Local Investors Portfolio Investment (LIPI) net flows by investor class and market segment.

- `nccpl-pp-cli lipi data` — Local Investors Portfolio Investment (LIPI) net flows by investor class and market segment. Date range.
- `nccpl-pp-cli lipi latest-date` — Most recent publication date available for lipi. Needs Cloudflare clearance only; no CSRF or session.

**lipi-normal** — LIPI buy/sell volume and value by client type and market type for one date.

- `nccpl-pp-cli lipi-normal data` — LIPI buy/sell volume and value by client type and market type for one date. Single settlement date.
- `nccpl-pp-cli lipi-normal latest-date` — Most recent publication date available for lipi-normal. Needs Cloudflare clearance only; no CSRF or session.

**lipi-sector** — LIPI net flows broken out by market sector.

- `nccpl-pp-cli lipi-sector data` — LIPI net flows broken out by market sector. Date range.
- `nccpl-pp-cli lipi-sector latest-date` — Most recent publication date available for lipi-sector. Needs Cloudflare clearance only; no CSRF or session.

**market** — Market-wide traded value and volume series.

- `nccpl-pp-cli market latest` — Most recent market-wide traded value or volume series. Needs Cloudflare clearance only.
- `nccpl-pp-cli market range` — Market-wide traded value or volume series over an explicit date range.

**mfs** — Murabaha Share Financing (MFS) open positions per symbol, with free-float percentages.

- `nccpl-pp-cli mfs data` — Murabaha Share Financing (MFS) open positions per symbol, with free-float percentages. Single settlement date.
- `nccpl-pp-cli mfs latest-date` — Most recent publication date available for mfs. Needs Cloudflare clearance only; no CSRF or session.

**mfs-top** — Top 15 MFS financee / financier pairs.

- `nccpl-pp-cli mfs-top data` — Top 15 MFS financee / financier pairs. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli mfs-top latest-date` — Most recent publication date available for mfs-top. Needs Cloudflare clearance only; no CSRF or session.

**msf** — Margin Sharia Financing (MSF) open positions per symbol.

- `nccpl-pp-cli msf data` — Margin Sharia Financing (MSF) open positions per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli msf latest-date` — Most recent publication date available for msf. Needs Cloudflare clearance only; no CSRF or session.

**msf-top** — Top 15 MSF buyer / seller pairs.

- `nccpl-pp-cli msf-top data` — Top 15 MSF buyer / seller pairs. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli msf-top latest-date` — Most recent publication date available for msf-top. Needs Cloudflare clearance only; no CSRF or session.

**mts** — Margin Trading System (MTS) open positions per symbol.

- `nccpl-pp-cli mts data` — Margin Trading System (MTS) open positions per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli mts latest-date` — Most recent publication date available for mts. Needs Cloudflare clearance only; no CSRF or session.

**mts-financiers** — Count of MTS financiers and financees per symbol.

- `nccpl-pp-cli mts-financiers data` — Count of MTS financiers and financees per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli mts-financiers latest-date` — Most recent publication date available for mts-financiers. Needs Cloudflare clearance only; no CSRF or session.

**mts-force-release** — MTS force-release volume and value by date.

- `nccpl-pp-cli mts-force-release data` — MTS force-release volume and value by date. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli mts-force-release latest-date` — Most recent publication date available for mts-force-release. Needs Cloudflare clearance only; no CSRF or session.

**mts-refinanced** — MTS amount released versus amount refinanced.

- `nccpl-pp-cli mts-refinanced data` — MTS amount released versus amount refinanced. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli mts-refinanced latest-date` — Most recent publication date available for mts-refinanced. Needs Cloudflare clearance only; no CSRF or session.

**mts-top-financiers** — Top 15 MTS financier / financee pairs.

- `nccpl-pp-cli mts-top-financiers data` — Top 15 MTS financier / financee pairs. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli mts-top-financiers latest-date` — Most recent publication date available for mts-top-financiers. Needs Cloudflare clearance only; no CSRF or session.

**settlement-cm** — Trade versus settlement volume and value, clearing-member-wise, per symbol.

- `nccpl-pp-cli settlement-cm data` — Trade versus settlement volume and value, clearing-member-wise, per symbol. Single settlement date.
- `nccpl-pp-cli settlement-cm latest-date` — Most recent publication date available for settlement-cm. Needs Cloudflare clearance only; no CSRF or session.

**settlement-uin** — Trade versus settlement volume and value, UIN-wise, per symbol.

- `nccpl-pp-cli settlement-uin data` — Trade versus settlement volume and value, UIN-wise, per symbol. Single settlement date.
- `nccpl-pp-cli settlement-uin latest-date` — Most recent publication date available for settlement-uin. Needs Cloudflare clearance only; no CSRF or session.

**slb** — Securities Lending and Borrowing (SLB) open positions per symbol.

- `nccpl-pp-cli slb data` — Securities Lending and Borrowing (SLB) open positions per symbol. Single settlement date.
- `nccpl-pp-cli slb latest-date` — Most recent publication date available for slb. Needs Cloudflare clearance only; no CSRF or session.

**tfc** — Unlisted Term Finance Certificate (TFC) transactions.

- `nccpl-pp-cli tfc data` — Unlisted Term Finance Certificate (TFC) transactions. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli tfc latest-date` — Most recent publication date available for tfc. Needs Cloudflare clearance only; no CSRF or session.

**var-margins** — Value-at-Risk margin, haircut and free float per symbol.

- `nccpl-pp-cli var-margins data` — Value-at-Risk margin, haircut and free float per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- `nccpl-pp-cli var-margins latest-date` — Most recent publication date available for var-margins. Needs Cloudflare clearance only; no CSRF or session.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
nccpl-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Backfill the foreign-flow archive and check it landed

```bash
nccpl-pp-cli sync --resources fipi,lipi --full && nccpl-pp-cli coverage --resources fipi,lipi --exit-code
```

Fills the local store as far back as NCCPL serves, then fails non-zero if any session is missing so a scheduled job cannot silently proceed on a hole.

### Emit a regression-ready flow panel with only the fields a model needs

```bash
nccpl-pp-cli panel --resource fipi --from 2015-12-09 --to 2026-09-04 --agent --select date,metric,value,observed_at
```

Long-format rows with the vintage stamp that establishes ex-ante availability, narrowed to four columns so an agent does not parse the whole payload.

### Find every symbol whose free float stepped this quarter

```bash
nccpl-pp-cli risk-changes --since 90d --field free_float --agent
```

Dates each change and prints symbols scanned versus symbols changed, so an empty result is distinguishable from an unscanned universe.

### Scan for leverage building ahead of a forced release

```bash
nccpl-pp-cli leverage --from 2026-08-01 --to 2026-09-04 --agent
```

One cross-section joining MTS, MFS and MSF open interest with SLB net open position, with force-release events flagged against the open interest they hit.

### Audit a date before trusting it

```bash
nccpl-pp-cli verify --from 2026-09-01 --to 2026-09-04 --agent
```

Checks FIPI net against minus LIPI net and each sector row's netting, returning a quarantine list of dates that fail.

## Auth Setup

NCCPL sits behind Cloudflare and a Laravel session, so there is no API key to set. Run 'nccpl-pp-cli auth login --chrome' to import the clearance and session cookies from a Chrome profile that has visited the site; the CLI then performs the CSRF handshake itself on each call. The Laravel session lasts about two hours and is renewed automatically, but the Cloudflare clearance cookie comes only from a real browser - if calls start returning challenge pages, re-run auth login.

Run `nccpl-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  nccpl-pp-cli fipi data --from-date 01/09/2026 --agent --select client_type,segment,buy_value
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `NCCPL_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `NCCPL_CONFIG_DIR`, `NCCPL_DATA_DIR`, `NCCPL_STATE_DIR`, `NCCPL_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `NCCPL_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `nccpl-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "nccpl": {
        "command": "nccpl-pp-mcp",
        "env": {
          "NCCPL_HOME": "/srv/nccpl"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `NCCPL_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `NCCPL_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
nccpl-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "nccpl-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `nccpl-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `nccpl-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
nccpl-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
nccpl-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
nccpl-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
nccpl-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`nccpl-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `NCCPL_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
nccpl-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
nccpl-pp-cli feedback --stdin < notes.txt
nccpl-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `NCCPL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NCCPL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
nccpl-pp-cli profile save briefing --json
nccpl-pp-cli --profile briefing fipi data --from-date 01/09/2026
nccpl-pp-cli profile list --json
nccpl-pp-cli profile show briefing
nccpl-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `nccpl-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/nccpl/cmd/nccpl-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add nccpl-pp-mcp -- nccpl-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which nccpl-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   nccpl-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `nccpl-pp-cli <command> --help`.

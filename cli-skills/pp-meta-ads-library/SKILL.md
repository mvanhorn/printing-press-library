---
name: pp-meta-ads-library
description: "Active public Meta ads worldwide (plus EU/UK/BR and political archives) from one static binary — zero browser, zero API key required, plus local SQLite competitive intelligence. Trigger phrases: `spy on competitor Meta ads`, `check Facebook Ad Library for a brand`, `find longest running Instagram ads`, `diff competitor ads since last week`, `map competitor ad landing pages and hooks`, `use meta-ads-library`, `run meta-ads-library`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - meta-ads-library-pp-cli
    install:
      - kind: go
        bins: [meta-ads-library-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/cmd/meta-ads-library-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/meta-ads-library/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Meta Ads Library — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `meta-ads-library-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install meta-ads-library --cli-only
   ```
2. Verify: `meta-ads-library-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.10 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/cmd/meta-ads-library-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search active and inactive commercial ads across Facebook, Instagram, Threads, Messenger, and Audience Network in any country via pure HTTP, with optional official Graph API v27.0 ads_archive lookups when META_ACCESS_TOKEN is set. Captures from ads search, ads details, diff --refresh, sov --live, and sync compound into local SQLite FTS5 tables powering winners, diff, compare, hooks, funnels, transparency, and sov (official archive queries Graph API independently).

## When to Use This CLI

Use Meta Ads Library CLI whenever you need competitive ad intelligence, creative teardowns, advertiser monitoring, landing-page funnel mapping, or EU/UK/political transparency audits across Facebook, Instagram, Threads, Messenger, and Audience Network. It runs zero-auth over pure HTTP without Chromium or Playwright and stores every search in local SQLite for instant offline FTS5 search, diffs, and cross-competitor comparisons.

## Anti-triggers

Do not use this CLI for:
- Managing, creating, pausing, or pulling private ROAS/CPA performance metrics from your own Meta Ads Manager account (use a Meta Marketing API CLI instead).
- Crawling or rendering third-party advertiser landing pages outside Meta Ad Library metadata.
- Querying US or LATAM commercial (non-political) ads via `archive`; Meta's official `ads_archive` excludes non-EU commercial ads (use `ads search` instead).

## Unique Capabilities

### Creative & funnel intelligence
- **`winners`** — Rank active ads by proven longevity (days running), collation scale, and platform breadth.

  _Reach for this when an operator asks which competitor creatives have been running longest or scaling hardest._

  ```bash
  meta-ads-library-pp-cli winners 15087023444 --min-days 30 --agent
  ```
- **`funnels`** — Group active ads by unwrapped destination domain, URL path, and UTM parameters including carousel cards.

  _Reach for this to map where a competitor sends paid traffic and which landing pages carry the most ad volume._

  ```bash
  meta-ads-library-pp-cli funnels 15087023444 --agent
  ```
- **`hooks`** — Extract, deduplicate, and rank opening copy hooks, headlines, and CTAs weighted by collation count and longevity.

  _Reach for this when preparing creative briefs or extracting the highest-volume opening hooks from synced ads._

  ```bash
  meta-ads-library-pp-cli hooks 15087023444 --agent
  ```

### Temporal tracking & benchmarking
- **`diff`** — See exactly which ads an advertiser launched, stopped, or scaled between timestamped snapshots.

  _Reach for this when monitoring what changed for a competitor since the last check._

  ```bash
  meta-ads-library-pp-cli diff 15087023444 --refresh --agent
  ```
- **`compare`** — Benchmark multiple advertisers side by side on active volume, median age, format mix, platforms, CTAs, and refresh velocity.

  _Reach for this when building a multi-competitor paid social benchmark or client audit._

  ```bash
  meta-ads-library-pp-cli compare 15087023444 182162001806727 --agent
  ```

### Transparency & market share
- **`transparency`** — Roll up EU/UK/BR reach, age-gender breakdowns, beneficiary/payer pairs, spend brackets, and violation flags per advertiser or byline.

  _Reach for this when auditing regulated EU/UK commercial spenders or political advertisers across all stored disclosures._

  ```bash
  meta-ads-library-pp-cli transparency 15087023444 --agent
  ```
- **`sov`** — Rank which advertisers dominate a category keyword by active ad count, collation-weighted volume, dominant format, and median longevity.

  _Reach for this to discover who owns paid share of voice in a niche before naming specific competitors._

  ```bash
  meta-ads-library-pp-cli sov "running shoes" --country US --live --agent
  ```

## Command Reference

**ads** — Zero-auth public Meta Ad Library search, transparency details, and deep links (pure HTTP, no API key required)

- `meta-ads-library-pp-cli ads search` — Search public Meta ads globally by keyword, page ID, brand name, or Ad Library URL
- `meta-ads-library-pp-cli ads details` — Fetch full ad details and EU/UK/BR transparency disclosures for a single ad (zero API key)
- `meta-ads-library-pp-cli ads url` — Build a clickable Meta Ad Library deep link for an ad ID, page ID, or keyword query


**advertisers** — Look up advertiser Facebook Pages and Page IDs in Meta Ad Library (zero API key)

- `meta-ads-library-pp-cli advertisers search` — Resolve a brand or advertiser name to its Facebook Page ID, likes, IG handle, and verification


**media** — Download ad creative images and videos from locally stored Meta ads

- `meta-ads-library-pp-cli media download` — Download creative images and videos for synced ads to a local folder


**archive** — Official Meta Ad Library Graph API v27.0 (ads_archive — political/issue ads globally and all EU/UK/BR ads; requires META_ACCESS_TOKEN)

- `meta-ads-library-pp-cli archive` — Query official Meta Ad Library Graph API v27.0 ads_archive for political/issue ads globally and all EU/UK/BR ads


**compare**


- `meta-ads-library-pp-cli compare` — Benchmark multiple advertisers side by side on active volume, median age, format mix, platforms, CTAs, and refresh velocity.


**diff**


- `meta-ads-library-pp-cli diff` — See exactly which ads an advertiser launched, stopped, or scaled between timestamped snapshots.


**funnels**


- `meta-ads-library-pp-cli funnels` — Group active ads by unwrapped destination domain, URL path, and UTM parameters including carousel cards.


**hooks**


- `meta-ads-library-pp-cli hooks` — Extract, deduplicate, and rank opening copy hooks, headlines, and CTAs weighted by collation count and longevity.


**sov**


- `meta-ads-library-pp-cli sov` — Rank which advertisers dominate a category keyword by active ad count, collation-weighted volume, dominant format, and median longevity.


**transparency**


- `meta-ads-library-pp-cli transparency` — Roll up EU/UK/BR reach, age-gender breakdowns, beneficiary/payer pairs, spend brackets, and violation flags per advertiser or byline.


**winners**


- `meta-ads-library-pp-cli winners` — Rank active ads by proven longevity (days running), collation scale, and platform breadth.




### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
meta-ads-library-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Compact competitor creative scan with --agent and --select

```bash
meta-ads-library-pp-cli ads search --query "running shoes" --country US --limit 10 --agent --select ad_archive_id,page_name,cta_text,landing_domain,collation_count,start_date
```

Fetch active US commercial ads and project only the high-signal creative and funnel fields to keep agent context compact.

### Weekly competitor change diff

```bash
meta-ads-library-pp-cli diff 15087023444 --refresh --country US --agent
```

Pull a fresh snapshot for a Facebook Page and diff it against the previous SQLite capture to see launched, stopped, and scaled ads.

### Side-by-side brand benchmark

```bash
meta-ads-library-pp-cli compare 15087023444 182162001806727 --agent
```

Compare multiple synced competitors on active volume, median ad age, format mix, placements, CTAs, and 7d/30d launch velocity.

### Category share-of-voice discovery

```bash
meta-ads-library-pp-cli sov "cold plunge" --country US --live --agent
```

Rank advertisers dominating a keyword by active ad share, collation-weighted creative volume, and median longevity.

### Official Graph API EU/political archive lookup

```bash
meta-ads-library-pp-cli archive --ad-reached-countries '["DE"]' --search-terms "nike" --limit 10 --agent --select id,page_name,eu_total_reach,spend,impressions,currency
```

Query Meta's official v27.0 ads_archive endpoint for regulated EU/UK/BR reach and political spend brackets when META_ACCESS_TOKEN is set.

## Auth Setup

No API key or browser is required for public commercial ad search (`ads search`), advertiser lookup (`advertisers search`), ad details (`ads details`), media downloads (`media download`), or local intelligence commands — the CLI bootstraps an anonymous GraphQL session automatically. To query Meta's official Graph API v27.0 `ads_archive` endpoint (`archive`) for political/issue spend ranges and EU/UK/BR reach datasets, export `META_ACCESS_TOKEN` (also accepts `META_ADS_ARCHIVE_TOKEN` or `FACEBOOK_ACCESS_TOKEN`) or persist it with `meta-ads-library-pp-cli auth set-token` (`auth status` and `auth logout` manage stored tokens). Note: `doctor` reports `META_ACCESS_TOKEN` as missing in zero-auth mode unless configured for `archive`.

Run `meta-ads-library-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array). `--csv` prefixes `'` on a cell or header key that starts with `=`, `+`, `-`, `@`, tab, CR, or LF (fullwidth `＝＋－＠` too) so a spreadsheet does not run it as a formula; a plain signed number such as `-5` or `+2.5e3` is unchanged.
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  meta-ads-library-pp-cli archive --ad-reached-countries '["US"]' --agent --select id,page_id,page_name
  ```
- **Previewable** — `--dry-run` shows the request without sending. A completed dry run exits 0 with `"success": true` and `"dry_run": true` and no HTTP `status`; validation failures still exit non-zero with `"success": false`.
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

- Use `--home <dir>` for one invocation, or set `META_ADS_LIBRARY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `META_ADS_LIBRARY_CONFIG_DIR`, `META_ADS_LIBRARY_DATA_DIR`, `META_ADS_LIBRARY_STATE_DIR`, `META_ADS_LIBRARY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `META_ADS_LIBRARY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `meta-ads-library-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "meta-ads-library": {
        "command": "meta-ads-library-pp-mcp",
        "env": {
          "META_ADS_LIBRARY_HOME": "/srv/meta-ads-library"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `META_ADS_LIBRARY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `META_ADS_LIBRARY_HOME`, or `doctor` will not find credentials left under the former root.

Local data is unencrypted and relies on OS account permissions plus disk encryption. Reads in the default `--data-source auto` mode are written to the SQLite store; GET responses are cached for five minutes under the cache dir. `local-data purge` previews what it would erase (store files, response cache, learn journal) and deletes only with `--yes`; it is not exposed over MCP. `META_ADS_LIBRARY_NO_LOCAL_DATA=1` disables the response cache and store writes, turns `auto` reads into live reads, and blocks `sync` and `workflow archive`; `--data-source local` still reads an existing store, and `META_ADS_LIBRARY_DATA_SOURCE` sets the `--data-source` default.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
meta-ads-library-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

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
      "next_action": ["<trial command>", "meta-ads-library-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `meta-ads-library-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `meta-ads-library-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `meta-ads-library-pp-cli sync --resources archive` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
meta-ads-library-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
meta-ads-library-pp-cli teach \
  --query "$QUERY" \
  --resource-type <type> \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
meta-ads-library-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
meta-ads-library-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
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

`meta-ads-library-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `META_ADS_LIBRARY_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
meta-ads-library-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
meta-ads-library-pp-cli feedback --stdin < notes.txt
meta-ads-library-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `META_ADS_LIBRARY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `META_ADS_LIBRARY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
meta-ads-library-pp-cli profile save briefing --json
meta-ads-library-pp-cli --profile briefing archive --ad-reached-countries '["US"]'
meta-ads-library-pp-cli profile list --json
meta-ads-library-pp-cli profile show briefing
meta-ads-library-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `meta-ads-library-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/cmd/meta-ads-library-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add meta-ads-library-pp-mcp -- meta-ads-library-pp-mcp
   ```
3. Verify: `claude mcp list`

For a remote host, `meta-ads-library-pp-mcp --transport http` requires `META_ADS_LIBRARY_MCP_HTTP_TOKEN` set to at least 32 characters (callers send `Authorization: Bearer <token>`; `openssl rand -hex 32` prints 64 hex characters and meets that minimum), binds `127.0.0.1:7777` by default, and needs `--tls-cert`/`--tls-key` for any non-loopback `--addr`. Requests with no `Origin` header are accepted. Loopback origins are accepted; any other `Origin` is refused unless listed in `META_ADS_LIBRARY_MCP_HTTP_ALLOWED_ORIGINS` (comma-separated `scheme://host[:port]` values). `META_ADS_LIBRARY_MCP_READ_ONLY=1` drops all mirrored tools without `readOnlyHint=true` (including local-store writers such as `teach`, `sync`, and `media download`) and makes the raw execute tool refuse state-changing calls.

## Direct Use

1. Check if installed: `which meta-ads-library-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   meta-ads-library-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `meta-ads-library-pp-cli <command> --help`.

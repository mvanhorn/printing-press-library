---
name: pp-autom
description: "Every Autom search engine from the terminal, plus saved rank history, cross-engine and cross-country comparison, and per-engine credit spend from a local ledger. Trigger phrases: `where does my site rank on Google`, `compare search results across engines`, `track keyword rankings`, `search Bing and Brave for`, `how many Autom credits are left`, `use autom`, `run autom`."
author: "Josselin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - autom-pp-cli
    install:
      - kind: go
        bins: [autom-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/autom/cmd/autom-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/autom/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Autom — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `autom-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install autom --cli-only
   ```
2. Verify: `autom-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.11 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/autom/cmd/autom-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search Google, Bing, Brave, DuckDuckGo and Yahoo through one key, with every documented parameter as a flag. rank check, batch run and the --save flag of the compare commands save each results page locally, so rank movers and visibility can answer what changed and where a domain stands, and credits shows where your credits went. Read-only: it searches and reads account usage; nothing on the Autom account is created or changed.

## When to Use This CLI

Use this CLI to fetch live search engine results from Google, Bing, Brave, DuckDuckGo or Yahoo, to track where a domain ranks over time, to compare rankings across engines or countries, or to monitor news about a brand. It is also the right tool to check Autom credit balance and spend before running large batches.

## Anti-triggers

Do not use this CLI for:
- Crawling or scraping the content of the result pages themselves; use a scraping API for that
- Keyword search volume or backlink data; Autom returns results pages only
- Writing or changing anything on an Autom account; the API is read-only apart from billing in the dashboard

## Unique Capabilities

### Search across engines and countries
- **`engines compare`** — See how one query ranks across Google, Bing, Brave, DuckDuckGo and Yahoo: shared domains, domains unique to each engine, and your domain's position on each. Costs 1 credit per engine.

  _Reach for this when a question is about visibility beyond Google or about which engines disagree._

  ```bash
  autom-pp-cli engines compare "serp api pricing" --engines google,bing,brave --domain autom.dev --agent
  ```
- **`geo compare`** — Run one query on one engine across several countries and see your domain's position and the shared results per country. Costs 1 credit per country.

  _Use this when the question is how rankings differ between markets._

  ```bash
  autom-pp-cli geo compare "serp api pricing" --countries fr,be,ca --domain autom.dev --agent
  ```

### Rank tracking from saved pages
- **`rank movers`** — List what moved across every tracked keyword since a cutoff: gains, drops, and entries into or exits from the top results. Reads the pages saved earlier by rank check, batch run or compare --save.

  _Use this for a weekly ranking report once pages have been saved on both sides of the cutoff; on a fresh store it returns a note._

  ```bash
  autom-pp-cli rank movers --since 7d --domain autom.dev --agent
  ```
- **`visibility`** — See one domain's footprint across every saved keyword: where it appears, average and best position, and top-3 and top-N (--top) counts, optionally against rival domains given with --vs. Reads the pages saved by rank check, batch run or compare --save.

  _Use this to compare a brand with competitors across a whole keyword set that you have already saved._

  ```bash
  autom-pp-cli visibility autom.dev --top 10 --agent
  ```

### Credit control
- **`credits`** — See your remaining quota, packs and credits plus where your credits went, by keyword, command or engine, with burn rate and days left. Spend covers only the calls made by this CLI; async searches show as pending.

  _Check this before and after large batches to predict spend and find the expensive workload._

  ```bash
  autom-pp-cli credits --by engine --since 30d --agent
  ```

### Monitoring
- **`news watch`** — Get only the news articles about a brand or keyword that earlier watches have not shown. Returned articles are marked as seen; --peek previews without marking. Costs 1 credit.

  _Schedule this for brand monitoring without re-reading the same articles._

  ```bash
  autom-pp-cli news watch "Autom" --country fr --agent
  ```

## Command Reference

**bing** — Bing web search

- `autom-pp-cli bing` — Query Bing Search via API and retrieve structured organic results (title, link, snippet, position).

**brave** — Brave web, image and video search

- `autom-pp-cli brave images` — Search Brave Images via API and retrieve structured image results with pagination and SafeSearch.
- `autom-pp-cli brave search` — Search Brave Search via API and retrieve structured organic results with pagination. Costs 1 credit per successful call.
- `autom-pp-cli brave videos` — Search Brave Videos via API and retrieve structured video results with pagination and SafeSearch.

**duckduckgo** — DuckDuckGo web search

- `autom-pp-cli duckduckgo` — Query DuckDuckGo via API and retrieve structured organic results with region and language.

**finder** — Free lookups of Google country, language and location codes

- `autom-pp-cli finder google-countries` — Search Google countries and retrieve country codes (free Finder endpoint).
- `autom-pp-cli finder google-languages` — Search Google languages and retrieve language codes (free Finder endpoint).
- `autom-pp-cli finder google-locations` — Find supported Google locations and retrieve location IDs and country codes (free endpoint).

**google** — Google search, news, images, videos, shopping, maps, autocomplete and link resolving

- `autom-pp-cli google autocomplete` — Get Google Search autocomplete suggestions via API for a given query. Costs 1 credit per successful call.
- `autom-pp-cli google images` — Search Google Images via API and retrieve image results (URLs, titles, sources, dimensions).
- `autom-pp-cli google maps` — Search Google Maps via API and retrieve structured place results (address, website, phone).
- `autom-pp-cli google news` — Search Google News via API and retrieve structured news results with pagination. Costs 1 credit per successful call.
- `autom-pp-cli google resolve` — Resolve google.com/goto links from organic search results. Experimental.
- `autom-pp-cli google search` — Scrape Google Search results via API and retrieve structured SERP data with pagination.
- `autom-pp-cli google search-light` — Scrape lightweight Google Search results via API: fast, cost-efficient, organic results only.
- `autom-pp-cli google shopping` — Scrape Google Shopping results via API and retrieve structured product listings with prices, ratings, and merchant data.
- `autom-pp-cli google videos` — Search Google Videos via API and retrieve structured video results with metadata. Costs 1 credit per successful call.

**job-results** — Read an async job by id

- `autom-pp-cli job-results <id>` — Poll the status, progress (0–100) and result of a job started with is_async=true. Polling is free.

**stats** — Free engine health over the last hour

- `autom-pp-cli stats` — Infrastructure health for an engine or endpoint over the last hour. Not scoped to your account or tenant. Needs --engine or --path; enabled per account (HTTP 403 means ask Autom support).

**usage** — Free account balance, quotas and rate limits

- `autom-pp-cli usage` — Retrieve real-time usage statistics and credit balance for your Autom account.

**web-search** — One multi-engine search

- `autom-pp-cli web-search` — One call walks engines in order and returns the first organic results.

**yahoo** — Yahoo web search

- `autom-pp-cli yahoo` — Query Yahoo via API and retrieve structured organic results with country and language.


**credits**


- `autom-pp-cli credits` — See your remaining quota, packs and credits plus where your credits went, by keyword, command or engine, with burn rate and days left. Spend covers only the calls made by this CLI; async searches show as pending.


**engines**


- `autom-pp-cli engines compare` — See how one query ranks across Google, Bing, Brave, DuckDuckGo and Yahoo: shared domains, domains unique to each engine, and your domain's position on each. Costs 1 credit per engine.


**geo**


- `autom-pp-cli geo compare` — Run one query on one engine across several countries and see your domain's position and the shared results per country. Costs 1 credit per country.


**news**


- `autom-pp-cli news watch` — Get only the news articles about a brand or keyword that earlier watches have not shown. Returned articles are marked as seen; --peek previews without marking. Costs 1 credit.


**rank**


- `autom-pp-cli rank movers` — List what moved across every tracked keyword since a cutoff: gains, drops, and entries into or exits from the top results. Reads the pages saved earlier by rank check, batch run or compare --save.
- `autom-pp-cli rank check` — Check where a domain ranks for a keyword on one engine and save the results page. Costs 1 credit.
- `autom-pp-cli rank history` — A domain's saved positions over time, per keyword. Reads saved pages; export with --csv.
- `autom-pp-cli rank competitors` — Who holds the top results for one keyword, and what changed since the previous saved page. Reads saved pages.


**visibility**


- `autom-pp-cli visibility` — See one domain's footprint across every saved keyword: where it appears, average and best position, and top-3 and top-N (--top) counts, optionally against rival domains given with --vs. Reads the pages saved by rank check, batch run or compare --save.




**serp**


- `autom-pp-cli serp list` — List saved results pages and their ids.
- `autom-pp-cli serp diff` — Which URLs entered, left or moved between two saved pages of a keyword.


**batch**


- `autom-pp-cli batch run` — Check a keyword list (file, stdin or --keywords) after checking the balance, and save every page. Costs 1 credit per distinct keyword/engine/country/lang.


**ground**


- `autom-pp-cli ground` — One numbered, deduplicated source list for a question across engines (and Google News with --news). Costs 1 credit per engine, plus 1 with --news.


**jobs**


- `autom-pp-cli jobs get` — Read an async job started with --is-async; --wait polls until it finishes. Polling is free.



### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
autom-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Narrow a Google search to links

```bash
autom-pp-cli google search --query "model context protocol" --agent --select organic_results.position,organic_results.link
```

Keep only positions and links from a large results payload. Costs 1 credit.

### Check and save a ranking

```bash
autom-pp-cli rank check autom.dev --query "serp api pricing" --country fr
```

Position of a domain for one keyword; the page is saved for rank history and movers. Costs 1 credit.

### Track a keyword list

```bash
autom-pp-cli batch run --keywords "serp api pricing,google search scraper" --domain autom.dev --agent
```

Checks the balance first, then saves every page. Costs 1 credit per distinct keyword/engine/country/lang.

### Weekly ranking report

```bash
autom-pp-cli rank movers --since 7d --domain autom.dev --agent
```

Gains, drops, entries and exits versus pages saved 7+ days ago by rank check or batch run. Free: reads saved pages.

### Compare markets

```bash
autom-pp-cli geo compare "serp api pricing" --countries fr,be,ca --domain autom.dev
```

Position per country and the results the countries share. Costs 3 credits (one per country).

### Brand news without repeats

```bash
autom-pp-cli news watch "Autom" --country fr
```

Only articles earlier watches have not shown, then marks them as seen. Costs 1 credit.

## Troubleshooting

- **Exit 7 on HTTP 401 "Rate limit exceeded"**: Autom signals rate limits with 401. Wait and retry, or lower `--concurrency` on `batch run`. A 401 mentioning a key quota means the key's request limits are used up; raise them in the dashboard or wait for the reset.
- **Exit 4 on HTTP 401 "Invalid API Key"**: set `AUTOM_API_KEY`, then confirm with `autom-pp-cli usage` (free).
- **HTTP 402**: out of credits. Check `autom-pp-cli credits`, top up, or enable Auto Top-Up.
- **Bing returns HTTP 500 when paging**: drop `--page` on `autom-pp-cli bing`.
- **Slow searches**: Autom can take up to 60 seconds; raise `--timeout`, or add `--is-async` and poll `autom-pp-cli jobs get <job-id> --wait`.
- **Not useful for this API**: `sync` has nothing to crawl (every endpoint needs a query), so `search` runs a billed live `web-search` in the default auto mode and only reads the store with `--data-source local`; `analytics`, `import`, `export` and `workflow archive` have no Autom data to work on. Use the saved-pages commands (rank history, rank movers, visibility, serp diff) for offline work.

## Auth Setup
Run `autom-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export AUTOM_API_KEY="<your-key>"
```
To persist credentials, use `echo "$TOKEN" | autom-pp-cli auth set-token`. Stored secrets live in `credentials.toml` under the data dir, not in `config.toml`.

Run `autom-pp-cli doctor` to verify setup.

`--ca-cert`, `AUTOM_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `AUTOM_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. `doctor` reports the trust in use and names the setting that fixes a chain or hostname failure.

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
  autom-pp-cli brave search --query "serp api pricing" --agent --select organic_results,search_parameters
  ```
- **Previewable** — `--dry-run` shows the request without sending. A completed dry run exits 0 with `"success": true` and `"dry_run": true` and no HTTP `status`; validation failures still exit non-zero with `"success": false`.
- **Offline history** — results pages saved by rank check, batch run and compare --save are read offline by rank history, rank movers, rank competitors, serp list/diff and visibility
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Read-only** — the CLI searches and reads account usage; nothing on the Autom account is created or changed

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

- Use `--home <dir>` for one invocation, or set `AUTOM_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `AUTOM_CONFIG_DIR`, `AUTOM_DATA_DIR`, `AUTOM_STATE_DIR`, `AUTOM_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `AUTOM_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `autom-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "autom": {
        "command": "autom-pp-mcp",
        "env": {
          "AUTOM_HOME": "/srv/autom"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `AUTOM_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `AUTOM_HOME`, or `doctor` will not find credentials left under the former root.

Local data is unencrypted and relies on OS account permissions plus disk encryption. Reads in the default `--data-source auto` mode are written to the SQLite store; GET responses are cached for five minutes under the cache dir. `local-data purge` previews what it would erase (store files, response cache, learn journal) and deletes only with `--yes`; it is not exposed over MCP. `AUTOM_NO_LOCAL_DATA=1` disables the response cache and store writes, turns `auto` reads into live reads, and blocks `sync` and `workflow archive`; rank check, batch run and compare --save then save nothing, news watch remembers nothing, and the credit ledger is not written; `--data-source local` still reads an existing store, and `AUTOM_DATA_SOURCE` sets the `--data-source` default.

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
autom-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "autom-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `autom-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `autom-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "images" → Google Images + Brave Images). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet. This CLI has no entity-lookup sync (every Autom endpoint needs a query), so do not run sync; add the mapping with `autom-pp-cli teach-lookup` when it matters.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
autom-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "ddg") satisfies future queries under another alias (e.g., "duckduckgo", "duck duck go") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
autom-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
autom-pp-cli teach-playbook \
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
autom-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, a result block that moved between pages, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-query / per-result data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`autom-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `AUTOM_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
autom-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
autom-pp-cli feedback --stdin < notes.txt
autom-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `AUTOM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AUTOM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Four sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `spacefast:<name>` | Publish the output as a live web page named `<name>`. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
autom-pp-cli profile save briefing --json
autom-pp-cli --profile briefing brave search --query "serp api pricing"
autom-pp-cli profile list --json
autom-pp-cli profile show briefing
autom-pp-cli profile delete briefing --yes
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
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `autom-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/autom/cmd/autom-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add autom-pp-mcp -- autom-pp-mcp
   ```
3. Verify: `claude mcp list`

For a remote host, `autom-pp-mcp --transport http` requires `AUTOM_MCP_HTTP_TOKEN` set to at least 32 characters (callers send `Authorization: Bearer <token>`; `openssl rand -hex 32` prints 64 hex characters and meets that minimum), binds `127.0.0.1:7777` by default, and needs `--tls-cert`/`--tls-key` for any non-loopback `--addr`. Requests with no `Origin` header are accepted. Loopback origins are accepted; any other `Origin` is refused unless listed in `AUTOM_MCP_HTTP_ALLOWED_ORIGINS` (comma-separated `scheme://host[:port]` values). `AUTOM_MCP_READ_ONLY=1` drops mirrored write commands (local-write commands that only touch the CLI's own learn store stay) and makes the raw execute tool refuse state-changing calls.

## Direct Use

1. Check if installed: `which autom-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   autom-pp-cli rank check autom.dev --query "serp api pricing" --agent
   ```
4. If ambiguous, drill into the group help, for example `autom-pp-cli rank --help` or `autom-pp-cli engines compare --help`.

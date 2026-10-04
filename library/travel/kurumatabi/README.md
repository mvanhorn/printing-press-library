# Kurumatabi / くるま旅 CLI

**Plan Japanese overnight vehicle stops with explicit fit, facilities and unknowns.**

Created by [@zjsng](https://github.com/zjsng) (zjsng).

Search the official RV Park and Kurumatabi ecosystem, inspect vehicle limits and facility fees, then compare evidence and prepare a host handoff. Cached observations support proximity and required-service screening without claiming live vacancies or dated stay totals.

## Install

The CLI is published in the travel catalog. This amendment awaits maintainer review. Build the proposed changes from its source checkout:

```bash
go build -o kurumatabi-pp-cli ./cmd/kurumatabi-pp-cli
./kurumatabi-pp-cli version
```

Install the currently published CLI from the catalog:

```bash
npx -y @mvanhorn/printing-press-library install kurumatabi --cli-only
```

## Authentication

Public search and detail require no account. The native search session is memory-only and is used only to preserve filter pagination.

## Quick Start

```bash
# Check the local CLI without network or credentials.
kurumatabi-pp-cli doctor --dry-run

# Discover a bounded official shortlist.
kurumatabi-pp-cli parks search --prefecture nagano --type rvpark --limit 5 --agent

# Read actual dimensions, tariffs and restrictions.
kurumatabi-pp-cli parks detail rvpark/1086 --agent

# Check published constraints against a real van.
kurumatabi-pp-cli parks fit rvpark/1086 --length-m 6 --width-m 2.2 --height-m 3 --vehicle van --membership nonmember --agent

# Prepare booking contact and unresolved questions.
kurumatabi-pp-cli parks handoff rvpark/1086 --agent

```

## Evidence contract

- `kind/numeric-id` and canonical URLs identify parks; Japanese names are preserved. `observed_at` uses JST; `source_updated_date_jst` is the provider's publication date, separately.
- Dimension values are metres and refer to the published parking space. Missing bounds are `null`; explicit unlimited height has its own flag. Extra-pitch/overhang permission requires host confirmation when a vehicle exceeds the nominal space.
- Facilities have `yes`, `no` or `unknown`, plus `free`, `paid`, `included` or `unknown` fee evidence. Disabled legacy icon filenames override positive alt labels. Mixed or conditional fee text stays unknown. A generic dump-station listing does not prove black/grey acceptance.
- Per-record membership conditions override broad type policy. RV Park and Kurumatabi Park generally allow nonmembers; other types and individual exceptions can restrict access. Member discounts alone do not prove a membership requirement.
- Tariffs preserve audience, basis and raw source text. They are published tariff cases, not dated totals. Same-day reservation acceptance is not vacancy. Broad opening labels do not override seasonal closure notes.
- A permitted overnight vehicle stay does not establish outdoor-camping permission. Specific activity rules remain in the detail evidence. Verify actual operation, pitch, dates, extras and rules with the host.

## Unique Features

These commands derive decisions across published evidence and cached observations.

### Vehicle and service decisions
- **`parks fit`** — Compare your vehicle with one park’s published dimensions and membership rules.

  _Large-motorhome service-stop planner can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks fit rvpark/1086 --length-m 6 --width-m 2.2 --height-m 3 --vehicle van --membership nonmember --agent
  ```
- **`parks match`** — Screen cached parks into proven, uncertain and ruled-out service matches.

  _Pet-owning van planner; Large-motorhome service-stop planner can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks match --require electricity,water,pets --membership nonmember --agent
  ```

### Observed park shortlists
- **`parks compare`** — Compare park evidence without inventing total prices or unknown facilities.

  _Pet-owning van planner; Club and nonmember tariff comparer can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks compare rvpark/1086 yypark/213 --agent
  ```
- **`parks near`** — Rank cached parks by straight-line distance from an explicit waypoint.

  _Pet-owning van planner can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks near --latitude 36.7 --longitude 138.2 --limit 5 --agent
  ```
- **`parks audit`** — Surface disabled-icon conflicts, stale observations and qualified opening notes.

  _Club and nonmember tariff comparer; Large-motorhome service-stop planner can act on explicit evidence and unresolved checks._

  ```bash
  kurumatabi-pp-cli parks audit rvpark/712 --agent
  ```

## Commands

| Command | Purpose |
|---|---|
| `parks filters` | Verified source labels and wire values |
| `parks search` | Prefecture/type/name/category/facility/period search with bounded provider pages |
| `parks detail` | Structured per-record dimensions, facilities, tariffs and rules |
| `parks handoff` | Public booking/contact links and unresolved questions |
| `parks fit` | One actual vehicle against one park's published evidence |
| `parks compare` | Evidence matrix for two to eight parks; fetch failures stay explicit |
| `parks near` | Straight-line distance ranking within the cached observed pool |
| `parks audit` | Disabled-icon conflicts, missing bounds, old observations and qualified rules |
| `parks match` | Candidate decisions and ID partitions: proven, uncertain or ruled out |

`source catalog` and `source page` are low-level HTML diagnostics. Normalized park observations are populated by `parks search`, `parks detail`, and other live park reads.

## Agent Usage

Use `--agent` for compact JSON and noninteractive output. Use explicit projections when a detail or comparison is large:

```bash
kurumatabi-pp-cli parks search --prefecture nagano --type rvpark --agent --select results.id,results.name,meta
kurumatabi-pp-cli parks detail rvpark/1086 --json --select results.id,results.dimensions,results.facilities,results.membership,meta
```

All list outputs use `[]` when empty. Missing dimension/coordinate evidence uses `null`; facilities, fee kinds and live vacancy use explicit unknown states. `--json`, `--agent`, `--compact`, `--select`, `--csv`, `--plain` and `--quiet` use the shared output pipeline. `--dry-run` validates the command shape without network or cache writes.

## Cache, coverage and bounds

`--data-source live` requires a real source response. `local` uses only cached observations. `auto` tries live and can return an existing cached observation after an ordinary source failure, with a stderr warning and `meta.source=local`; throttling is always an error. The command timeout still bounds the work.

Search performs one native POST with a memory-only search cookie jar, then GET pagination; no browser runs during CLI use. `--limit` bounds output (1..100); `--max-scan-pages` bounds provider work (1..5 pages, normally 20 cards each). Metadata reports provider total, scanned rows/pages, output truncation and whether provider pages were exhausted. The total is a source observation, not a catalog-size promise.

Local search, near and match cover only the cached observed pool. Near/match reject live mode, have separate `--max-scan-records` and `--limit` bounds, and report their scanned denominator. Near reports missing coordinates and uses Haversine distance, not road distance. Match checks requested facilities and optional exact fee kinds; unknowns never become proven matches, and card-only observations require detail confirmation.

Cache records are stored in SQLite. Search refreshes never overwrite a richer detail observation. `--db` selects an explicit cache; otherwise the runtime data path follows the CLI home/path settings. Use `kurumatabi-pp-cli agent-context --json` for current paths. `--no-cache` skips live park cache writes and automatic fallback; `--no-learn` disables the generated local learning loop.

## Health Check

```bash
kurumatabi-pp-cli doctor --json
kurumatabi-pp-cli parks filters --json
```

## Cookbook

```bash
# Read both general/member tariff cases without calculating an unverified total.
kurumatabi-pp-cli parks compare rvpark/1086 yypark/213 --json
# Check disabled-icon evidence before relying on a legacy listing.
kurumatabi-pp-cli parks audit rvpark/712 --json
# Rank an observed pool after populating detail records.
kurumatabi-pp-cli parks near --latitude 36.7 --longitude 138.2 --radius-km 100 --limit 5 --agent
# Screen services and exact fee evidence; every candidate has predicate proofs.
kurumatabi-pp-cli parks match --require electricity=free,water,pets --membership nonmember --agent
# Search only local observations; this never claims nationwide completeness.
kurumatabi-pp-cli parks search --keyword 黒姫 --data-source local --json
```

## Troubleshooting

| Symptom | Action |
|---|---|
| Empty local pool | Run `parks detail rvpark/1086` or a bounded `parks search`; keep the same `--db` |
| Incomplete search coverage | Inspect metadata and raise `--max-scan-pages` within its cap |
| Unknown fit or service decision | Read the predicate evidence and confirm that condition with the host |
| Invalid source label | Run `parks filters`; local region queries require a specific prefecture |
| HTTP or markup failure | Open the canonical source page and check whether the public contract changed |
| Rate limit | Back off and retry later; no empty-result or cached success is fabricated for a 429 |

Exit codes used by domain commands: 0 success; 2 invalid input; 3 missing cached ID; 5 source/API failure, including a partial comparison; 7 throttling. Generated framework commands have additional documented codes; inspect their help.

## Validation and maintenance

Source discovery used a dedicated native CUA tab and public DOM/forms, followed by direct HTTP replay. No login, CAPTCHA, CDP or provider-restriction bypass was used. Tests cover modern/legacy icons, explicit wastewater negatives, conditional fees, units, membership, native session pagination, exact opening labels, missing/updated caches, fallback, fit, comparison, proximity, audit and required-service decisions.

The source includes an MCP build target exposing the nine park planning commands over local stdio. Build it with `go build -o kurumatabi-pp-mcp ./cmd/kurumatabi-pp-mcp`. Raw HTML source endpoints are CLI diagnostics and are excluded from MCP. No remote service or MCP listener is deployed by this task.

## Recipes

### Bounded park shortlist

```bash
kurumatabi-pp-cli parks search --prefecture nagano --type rvpark --agent --select results.id,results.name,meta
```

Keep identifiers, Japanese names and source coverage.

### Vehicle evidence

```bash
kurumatabi-pp-cli parks fit rvpark/1086 --length-m 6 --width-m 2.2 --height-m 3 --vehicle van --membership nonmember --agent
```

Unknown constraints require host confirmation.

### Park comparison

```bash
kurumatabi-pp-cli parks compare rvpark/1086 yypark/213 --agent
```

Tariff audiences and fees remain separate.

### Offline proximity

```bash
kurumatabi-pp-cli parks near --latitude 36.7 --longitude 138.2 --limit 5 --agent
```

Ranks only cached observed parks; distance is straight line.

### Required services

```bash
kurumatabi-pp-cli parks match --require electricity,water,pets --membership nonmember --agent
```

Unknown evidence never becomes a proven match.

Source HTML/JSON response-body reads and each explicit decompression layer are bounded to 4 MiB. Go transparent gzip is already decoded when read through the transport body. A required comparison fetch failure emits the usable partial envelope once and exits 5. Agent field selection always retains source/coverage metadata and the top-level `results` field; metadata-only selection returns `results: []`.

A partial comparison is emitted on stdout and returns exit5. Explicit file/webhook delivery occurs only after success; failed comparisons do not deliver the partial buffer.

Search caches every observation in the bounded page scan before applying the returned-row limit. Detail, fit, compare, audit and handoff require detailed observations for local reads or automatic fallback; refresh a search-only card with `parks detail ID --data-source live`. Multiple bath/onsen icons retain every source observation and qualified fee evidence.

MCP comparison failures retain bounded partial JSON in the first content block, with `isError: true` and a separate diagnostic capped at 4,000 bytes. Oversized evidence uses an explicit preview; total failure text is bounded to 64,000 bytes. `learnings forget` atomically removes matched teachings and reconciles existing inferred query/resource/venue families against all retained evidence. Supported rules keep their IDs and refreshed examples; unsupported rules are removed. Manually taught and unrelated rules remain. Explicit `teach-pattern` updates replace the complete recorded scope and examples; later inference preserves that manual definition. Verified prefix patterns require one literal match, including `%` and `_` characters.

Native park MCP mirrors and the comparison recipe force `--no-learn` after caller arguments, preventing automatic journaling or preference derivation. Explicit recall and local-writing tools retain their own behavior. Ordinary CLI learning remains optional through the documented controls. Native reads can still refresh the evidence cache; `--no-cache` disables those cache writes and fallback.

`export source` exports canonical link objects from the first catalog page as JSONL or a JSON array, with a limit of 1 through 200 links. It does not export detailed records or claim nationwide coverage. Invalid options and failed source reads leave a destination untouched; dry runs do not create output files. Source redirects must stay within the requested effective origin; foreign host, port or scheme changes are refused before custom credentials can be sent.

MCP hints reflect durable effects: filters, near and match are read-only and do not expose receipt writes; search, detail, fit, compare, audit, handoff and the comparison recipe may refresh the evidence cache and advertise local writes. Explicit learning readers and workflow status also advertise local writes because their normal CLI paths can update learning/journal state. Typed context/SQL tools remain read-only. The named variadic `park-id` input for `parks_compare` accepts a whitespace-separated ID list; scalar text inputs remain one argument, and flag-like tokens are rejected.

### Catalog links and safe agent recall

```bash
kurumatabi-pp-cli export --format json --limit 20 --no-learn source
kurumatabi-pp-cli parks compare rvpark/1086 yypark/213 --agent --no-learn
```

The catalog export covers first-page link objects only. MCP named comparison input uses `park-id: "rvpark/1086 yypark/213"`; scalar recall questions remain one argument. Framework recall checks actual cached resource identity before promoting aliases. A known conflicting identity stays a mismatch; a missing resource stays explicitly warned and still requires fresh source verification.

Source and agent capabilities include bounded catalog export, truthful MCP local-state hints, safe named comparisons and evidence-aware alias recall in addition to the five domain feature groups above.

Synthesized recall checks any extractable cached identity too; a known conflicting target stays a mismatch. Identifier-verified patterns with no extractable identity retain their existing behavior without invented entities, and every recalled travel ID still needs fresh source verification.

Synthesized identity validation follows the exact entity substituted into its resource ID; another entity in a compound query cannot validate a conflicting target. Forgetting a teaching preserves an inferred rule only when at least two distinct compatible bindings remain, ignores unusable rows, and repairs examples from those compatible supporters.

Recall limits apply after cached identity validation and accepted-ID deduplication, so rejected candidates do not consume the requested result count.

Recall checks later bindings when an earlier one conflicts, selects one compatible binding per pattern, and ranks before deduplicating typed resource IDs. Accepted targets are removed from contradictory mismatch alternatives. `--limit` caps returned results and diagnostic rows; it does not promise a cap on local validation work, which depends on the learned pattern store. Full validation preserves confidence and identity ranking.

Only a genuinely absent cached resource uses the warned missing-row fallback. Cancellation and unreadable cached payloads fail the command; they do not produce an exact result.

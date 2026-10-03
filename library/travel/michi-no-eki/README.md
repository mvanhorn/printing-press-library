# Michi-no-Eki

Created by [@zjsng](https://github.com/zjsng) (zjsng).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

**Find and compare Japan roadside-station service stops with source evidence and explicit unknowns.**

Search the official Japanese directory by prefecture, source region, and facilities. Compare parking and published hours, rank nearby candidates, inspect bounded station notices, and compare saved observations without inferring camping permission.

## Install

After this addition merges and the catalog updates, install the CLI with:

```bash
npx -y @mvanhorn/printing-press-library install michi-no-eki --cli-only
michi-no-eki-pp-cli --version
```

For Go source installation:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/cmd/michi-no-eki-pp-cli@latest
```

For review before merge, build from this CLI directory with `go build ./cmd/michi-no-eki-pp-cli`.

## Authentication

Authorized directory routes require no authentication. No accounts, cookies or bookings are used.

## Quick Start

```bash
# Check local command readiness without network.
michi-no-eki-pp-cli doctor --dry-run

# Resolve official prefecture and facility vocabulary.
michi-no-eki-pp-cli catalog --agent

# Find a bounded source-listed bath shortlist.
michi-no-eki-pp-cli find --prefecture nagano --facility onsen --limit 3 --agent

# Inspect facts and unresolved overnight/current-service questions.
michi-no-eki-pp-cli readiness 19187 --agent

```

## Known Gaps

The twelve domain commands and saved-snapshot workflow are verified. Optional generated framework helpers have these limits:

- `export bulletins` writes parsed notice records (JSON or JSONL) from the `notices`/`notice` parser. `--limit 0` follows the notice index until it ends; a positive `--limit` stops at that many records. Export fails instead of writing a file when another index page remains after 500 pages. A failed export leaves an existing `--output` file unchanged. It does not emit provider HTML. Use `snapshot --ids ... --json` for factual station observations.
- SQL/workflow-status text refers to sync, but this source has no domain sync or database-ingestion command.
- Generic list/search link `image` fields can contain an HTML page URL when no image exists; the authored domain/detail workflows do not use that field.
- Explicit optional learning helpers disclose their local writes: `recall` and playbook listing append usage/audit records, and `learnings stats` can migrate the store and prune local telemetry, including with `--no-learn`. Native domain calls and low-level stations/bulletins reads suppress automatic learning journals/corrections. Source requests require no credentials. Credential-like custom headers such as `X-API-Key` are withheld on cross-origin redirects; do not configure other sensitive headers. Declared HTML requests are bounded to 5 MiB during wire reading and decompression; ordinary API/error bodies are bounded to 32 MiB. Successful generic binary envelopes retain their separate semantics.

Raw typed MCP HTML tools are hidden by the source spec. The optional runtime catalog has 29 tools including 12 domain mirrors; generated 4/4/full metadata describes four endpoint definitions and their no-auth readiness, not that runtime total. CLI-only installation leaves MCP installation to the user. The optional MCP mirrors for compare, snapshot and station-notices preserve bounded structured failure accounting in an explicitly failed tool result; they do not rerun the source operation. These generated limitations are retained as explicit template review evidence.

## Commands

| Command | Purpose |
|---|---|
| `catalog` | Source prefecture/facility IDs, Japanese labels, English slugs and provider region groupings |
| `find` | Bounded service-stop discovery by area, keyword and facilities |
| `station [id]` | Parking capacities, published hours, visible phone, operator links and facility evidence |
| `notices` | Dated notice titles and canonical links within page scan coverage |
| `notice [id]` | Bounded Japanese excerpt, publication date and exact linked station IDs |
| `readiness [id]` | Evidence plus unresolved permission, service, fee and vehicle-fit questions |
| `compare --ids` | Ordered factual station comparison with per-ID failures |
| `nearby --lat --lon` | Straight-line kilometers within explicit source query/candidate coverage |
| `station-notices [id]` | Exact station-ID links within bounded index/detail scans |
| `snapshot --ids` | Factual JSON observation to stdout for user-directed saving |
| `changes --before --after` | Offline stable-ID factual/membership diff; timestamps excluded |
| `guidance` | Attributed general MLIT rest/lodging/camping distinctions |

The generated `stations search/get` and `bulletins list/get` commands provide low-level HTML metadata/links. Prefer the domain commands above for campervan planning. Generic framework learning/config/schema commands remain available through help; the domain snapshot workflow uses JSON files and needs no nationwide sync.

## Unique Features

Native station planning preserves read-only source behavior. Optional local learning has explicit write hints, validates cached resource identity, and preserves supported derived rules on targeted undo.

These commands combine source evidence into bounded planning workflows.

### Accountable campervan stops
- **`readiness`** — Inspect source facts and permission or service unknowns for one station ID

  _Campervan planner; trip agent can inspect source evidence without merging tabs manually._

  ```bash
  michi-no-eki-pp-cli readiness 19187 --agent
  ```
- **`compare`** — Compare up to six ordered station IDs with source facts and per-ID failures

  _Campervan planner; trip agent can inspect source evidence without merging tabs manually._

  ```bash
  michi-no-eki-pp-cli compare --ids 19187,19189 --agent
  ```
- **`nearby`** — Rank source stations by straight-line kilometers from required latitude/longitude, scoped by area and facilities

  _Campervan planner can inspect source evidence without merging tabs manually._

  ```bash
  michi-no-eki-pp-cli nearby --prefecture nagano --facility onsen --lat 35.8632686 --lon 138.2769108 --limit 3 --agent
  ```
- **`station-notices`** — Find notices explicitly linked to one station ID within page/detail scan limits

  _Advisory researcher; trip agent can inspect source evidence without merging tabs manually._

  ```bash
  michi-no-eki-pp-cli station-notices 19487 --max-scan-pages 1 --max-detail-records 10 --agent
  ```
- **`changes`** — Compare two saved snapshot files offline by stable station ID and factual fields, or use an explicit synthetic demo

  _Trip agent; advisory researcher can inspect source evidence without merging tabs manually._

  ```bash
  michi-no-eki-pp-cli changes --demo --agent
  ```

## Agent Usage

Use `--agent` or `--json` for one JSON value, `--select` to project fields, and `--compact` when a reduced response is sufficient. Lists are initialized as `[]`; source errors, challenge shells and unknown layouts produce errors, not empty success. `--dry-run` performs no domain network/file work. Live domain commands reject `--data-source local`; `changes` rejects live mode.

```bash
michi-no-eki-pp-cli compare --ids 19187,19189 --agent --select stations.id,stations.name,stations.parking,fetch_failures
michi-no-eki-pp-cli find --prefecture nagano --facility onsen --limit 3 --agent --select stations.id,stations.name,scope,source_reported_total
```

`find`/`nearby` return at most 50 rows; `--max-candidates` independently caps candidate examination (default 2000, maximum 5000). Declared HTML transport bounds wire bytes and each decode layer to 5 MiB; the domain parser rechecks that bound before parsing. This is a body-byte bound, not a total process RSS cap. Comparisons/snapshots accept at most 6 unique station IDs. `notices`/`station-notices` scan at most 5 index pages. `station-notices` retains every notice on those pages, then opens at most `--max-detail-records` detail pages (default 10, maximum 50). A station link after that detail prefix is not a match. Root `--timeout` bounds the whole live domain workflow; generated request pacing applies, and domain observations bypass the response cache. Partial failures remain in `fetch_failures` and are reported to stderr. All-failure operations exit nonzero.

## Health Check

```bash
michi-no-eki-pp-cli doctor --json
michi-no-eki-pp-cli station 19187 --json
```

`doctor` checks generic HTTP/tool readiness. A real `station`/`find` parse verifies the current provider HTML structure; a successful homepage response alone does not validate every feature.

## Cookbook

### Inspect optional local learning evidence

```bash
michi-no-eki-pp-cli recall "station 19187" --agent --debug-mismatches
michi-no-eki-pp-cli learnings list --agent

# Only when the user asks to undo this specific local teaching
michi-no-eki-pp-cli learnings forget "station 19187" --resource 19187 --agent
```

These optional helpers write local learning/audit state; they do not fetch station evidence or populate a nationwide station cache. A known conflicting cached resource remains in `mismatches` and cannot become an exact hit through a teaching alias or synthesized pattern. Direct aliases require a canonical shared by query, teaching and cached resource. Pattern validation uses the exact entity that produced its candidate, not unrelated entities elsewhere in the query. Recall considers later verified entity bindings from the same pattern when an earlier binding conflicts, retains its first accepted binding, and accepted typed resource IDs are omitted from mismatch diagnostics and their warnings. Final typed-ID deduplication retains the best validated hit under Recall ranking, so a prior partial or lower-ranked hit cannot hide better evidence. `recall --limit` caps final results, not local database work: candidate validation can scan the full optional pattern store to preserve identity filtering and final ranking. Large local stores can take longer; no local-work or latency bound is claimed. Identifier-verified patterns with no extractable cached identity retain their legacy matching behavior; that is not evidence of current provider identity or facts. Missing cached resources retain an explicit warning and still require a live source fetch. Only a genuine missing row uses this fallback; unexpected cached-identity read failures or cancellation fail recall with runtime exit 5 and no success envelope. Undo reconciles only affected inferred families. A rule needs two distinct compatible positive (`boost`) examples from eligible teaching sources; unusable rows do not veto valid support. New synthesis and reconciliation after an affected forget exclude hide/alias effects. Unsupported affected rules are removed; explicitly taught and unrelated patterns remain. An explicit `teach-pattern` records its full declared scope; later inference cannot replace it.

Native domain calls and low-level stations/bulletins reads suppress automatic learning journals/corrections. The six stateful MCP helpers disclose local writes; normal optional recall results remain available.


```bash
# Bath and food candidates; listed presence only
michi-no-eki-pp-cli find --prefecture nagano --facility onsen,restaurant --match all --limit 5 --json

# Nearby source candidates, straight-line kilometers
michi-no-eki-pp-cli nearby --prefecture nagano --facility onsen --lat 35.8632686 --lon 138.2769108 --limit 3 --json

# Exact station-linked publication coverage
michi-no-eki-pp-cli station-notices 19487 --max-scan-pages 1 --max-detail-records 10 --limit 5 --json

# User-directed JSON saving and offline comparison
michi-no-eki-pp-cli snapshot --ids 19187,19189 --json > before.json
michi-no-eki-pp-cli snapshot --ids 19187,19189 --json > after.json
michi-no-eki-pp-cli changes --before before.json --after after.json --json

# Explicit synthetic demonstration; not a real station observation
michi-no-eki-pp-cli changes --demo --json
```

## Evidence and Limits

An active facility icon means `present` in the source listing; an inactive icon means `not_listed`, rather than proof of physical absence. Missing/unknown icon data stays `unknown`. Published hours are reference text, parking capacities are nominal vehicle counts, and fees/current opening/charger operation/parking vacancies/campervan fit remain unknown unless explicitly sourced. Operator URLs are handed off without scraping unrelated sites.

A directory listing, campground/lodging icon or large parking count does not establish vehicle lodging or outdoor-camping permission. [MLIT's parking FAQ](https://www.mlit.go.jp/road/soudan/soudan_03_04.html) distinguishes fatigue-recovery rest/napping from public-space lodging use and notes that some stations offer separately designated lodging parking. Confirm the individual station's spaces, dates and rules. `guidance` records the verification date; it is a general reference, not a station permission decision.

Nearby distances are straight-line kilometers, not road routes or vehicle-clearance guarantees. Invalid/missing coordinates are excluded and counted. Source headline counts can differ from dated registration notices; this CLI reports the observed query count/card coverage and makes no national completeness claim. When the provider renders an explicit empty map-data dataset without a count, `source_reported_total` is null and `empty_result_evidence` explains the observed shape.

Notice publication dates are normalized to `YYYY-MM-DD` in Asia/Tokyo, while event/service dates stay in the bounded original Japanese excerpt. `station-notices` uses explicit linked IDs and only examined detail pages; no match does not mean no closure or advisory exists. Excerpts are capped at 160 characters and full text/images/attachments remain at the canonical source. Captured full pages are private build evidence; packaged tests use synthetic markup. [Provider site terms](https://www.michi-no-eki.jp/about/term) govern source content; this local build is not a redistribution of their site.

## Troubleshooting

| Symptom | Action |
|---|---|
| Unknown filter or region | Run `catalog`; use a slug, Japanese label or official ID |
| No all-of facility matches | Inspect `scope`, `matched_count` and `scanned_candidates`; try `--match any` if appropriate |
| No station-linked notices | Inspect page/detail counts and publication range; widen scan caps within the timeout |
| Provider layout unrecognized | Open the reported source URL and re-check the parser; do not treat it as no data |
| HTTP throttling/unavailability | Respect the error and retry later; reduce scans or configure slower pacing |
| Local file is not a snapshot | Save complete `snapshot --ids ... --json` output; projected outputs are not full snapshots |
| Live mode on `changes` | Remove `--data-source live`; snapshots are local observations |

Exit codes follow the generated runtime:0 success;2 usage/input;3 not found;4 auth;5 API/runtime;7 rate limiting;10 configuration. A partial result can succeed with explicit failures; an all-failure operation does not.

## Development

```bash
go test -count=1 ./...
go vet ./...
go build ./...
```

Public-source contract research and local Printing Press evidence accompany this build; publishing, bookings, accounts and external mutations are outside scope. License:Apache-2.0.

## Recipes

### Compare stop evidence

```bash
michi-no-eki-pp-cli compare --ids 19187,19189 --agent --select stations.id,stations.name,stations.parking,fetch_failures
```

Return ordered station facts and explicit fetch gaps.

### Nearby bath stops

```bash
michi-no-eki-pp-cli nearby --prefecture nagano --facility onsen --lat 35.8632686 --lon 138.2769108 --limit 3 --agent
```

Straight-line distance ranks scoped source candidates, not driving routes.

### Station notice watch

```bash
michi-no-eki-pp-cli station-notices 19487 --max-scan-pages 1 --max-detail-records 10 --agent
```

Match exact linked station IDs within a bounded notice scan.

### Offline observation changes

```bash
michi-no-eki-pp-cli changes --before before.json --after after.json --agent
```

Compare user-saved factual observations without network.

## Generated Framework Limitations

The package provides the CLI and agent skill. Typed MCP HTML endpoint tools are hidden by the source spec because their generated handlers expose raw HTML. Domain CLI workflows and their command mirrors provide factual extraction. CLI-only installation does not activate the optional MCP server.

The provider requires no credentials. Credential-like headers such as `Authorization` and `X-API-Key` are withheld on a cross-origin redirect; other custom headers can still be copied, so do not configure sensitive non-credential names. Normal source requests use no credential headers. The generated HTTP transport requires HTTP/2; the public provider was verified with that transport.

Generated low-level link rows can place an HTML page URL in `image` when no image is published; treat that field as unverified. Use normalized domain workflows for service evidence. Detail endpoint commands return page metadata rather than an empty link-only result.

Low-level detail metadata includes the requested numeric ID, canonical handoff URL, source URL and JST observation time. `entity_fields_status` distinguishes recognized native identity fields from generic page metadata with explicit unknowns. Domain `station`/`notice` remain the richer factual workflows.

Optional framework SQL/workflow-status descriptions mention sync, but this source has no domain sync or database-ingestion workflow; use native domain commands and saved snapshots. Explicit optional learning helpers can open/create/migrate local stores or append audit/telemetry. Six MCP helpers (recall, learnings candidates/list/stats, playbook list and workflow status) declare those local writes with readOnly=false; direct SQL/context remain read-only. Native domain commands, low-level stations/bulletins reads and help suppress automatic journals/corrections. `--no-learn` does not make explicit optional learning helpers write-free. CLI-only installation does not activate the optional MCP server.

Low-level list/search reads in `--data-source auto` may initialize or migrate the optional resolver cache even when an HTML response yields no cacheable JSON records. `--no-cache` controls the HTTP cache; use `--data-source live --no-cache` to bypass resolver write-through and HTTP caching. The detail metadata adapters use direct source reads. Low-level groups suppress automatic learning journals/corrections; this cache behavior is separate from the twelve native commands and help paths that leave no automatic local state.

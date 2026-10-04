---
name: pp-michi-no-eki
description: "Find and compare Japan roadside-station service stops with source evidence and explicit unknowns. Trigger phrases: `find Nagano roadside stations with onsen`, `compare roadside-station parking`, `nearby Michi-no-Eki service stops`, `check station-linked notices`, `use michi-no-eki`, `run michi-no-eki`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - michi-no-eki-pp-cli
---

# Michi-no-Eki source workflow

## Prerequisites: Install the CLI

This skill drives the `michi-no-eki-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install michi-no-eki --cli-only
   ```
2. Verify: `michi-no-eki-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/cmd/michi-no-eki-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search the official Japanese directory by prefecture, source region, and facilities. Compare parking and published hours, rank nearby candidates, inspect bounded station notices, and compare saved observations without inferring camping permission.

## When to Use This CLI

Find Japan roadside-station service stops; inspect source-listed food, shops, baths, parking and facilities; compare a shortlist; rank scoped nearby candidates; inspect bounded station-linked notices; compare saved factual observations offline. Resolve filter vocabulary with `catalog`, then use concrete IDs from `find`.

## Anti-triggers

Use the relevant booking/provider interface for campsite/RV inventory or transactions. Use routing tools for driving/vehicle-clearance planning. Station listing or active icons do not establish overnight lodging, outdoor camping or live service availability.

## Unique Capabilities

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

## Source workflow

1. Run `catalog` for official IDs/aliases. Choose `--prefecture` or the provider's `--region`, not both. Preserve Japanese names and canonical URLs.
2. Use `find --facility` with explicit `--match all|any`. Provider multi-facility search is any-of; the default all-of is a local required-facility filter. `present` is listed evidence, `not_listed` is an inactive source icon, and missing data is `unknown`.
3. Use `station`, `compare` or `readiness` for published hours, nominal parking capacities and operator handoff links. Current opening, fees, charging operation, spaces and vehicle fit remain unknown. General MLIT rest guidance stays separate from per-station permissions.
4. Use `nearby` only with supplied itinerary coordinates; distances are straight-line kilometers. Inspect candidate cap/coverage and missing-coordinate warnings.
5. Use `notices` or `station-notices` with independent page/detail scan caps. Match exact linked station IDs. `station-notices` retains every notice on the scanned index pages, then opens only the `--max-detail-records` prefix; a no-match covers those opened details, not later index rows. Publication date is a JST calendar date, distinct from dates discussed in text. Follow the canonical notice for full text/attachments.
6. Use complete `snapshot --ids ... --json` stdout for user-directed local saving. `changes` reads two snapshots, excludes observation timestamps, and marks fetch gaps unresolved. Projected outputs cannot serve as complete snapshots.

## Command Reference

Use `find`, `station`, `notices`, `notice`, `catalog`, `guidance`, `snapshot`, and the verified capability commands above. Each command exposes domain examples in help. Low-level `stations search/get` and `bulletins list/get` return HTML metadata/links; prefer domain commands for planning.

## Recipes

### Inspect optional local learning evidence

```bash
michi-no-eki-pp-cli recall "station 19187" --agent --debug-mismatches
michi-no-eki-pp-cli learnings list --agent

# Only when the user asks to undo this specific local teaching
michi-no-eki-pp-cli learnings forget "station 19187" --resource 19187 --agent
```

These optional helpers write local learning/audit state; they do not fetch station evidence or populate a nationwide station cache. A known conflicting cached resource remains in `mismatches` and cannot become an exact hit through a teaching alias or synthesized pattern. Direct aliases require a canonical shared by query, teaching and cached resource. Pattern validation uses the exact entity that produced its candidate, not unrelated entities elsewhere in the query. Recall considers later verified entity bindings from the same pattern when an earlier binding conflicts, retains its first accepted binding, and accepted typed resource IDs are omitted from mismatch diagnostics and their warnings. Final typed-ID deduplication retains the best validated hit under Recall ranking, so a prior partial or lower-ranked hit cannot hide better evidence. `recall --limit` caps final results, not local database work: candidate validation can scan the full optional pattern store to preserve identity filtering and final ranking. Large local stores can take longer; no local-work or latency bound is claimed. Identifier-verified patterns with no extractable cached identity retain their legacy matching behavior; that is not evidence of current provider identity or facts. Missing cached resources retain an explicit warning and still require a live source fetch. Only a genuine missing row uses this fallback; unexpected cached-identity read failures or cancellation fail recall with runtime exit 5 and no success envelope. Undo reconciles only affected inferred families. A rule needs two distinct compatible positive (`boost`) examples from eligible teaching sources; unusable rows do not veto valid support. New synthesis and reconciliation after an affected forget exclude hide/alias effects. Unsupported affected rules are removed; explicitly taught and unrelated patterns remain. An explicit `teach-pattern` records its full declared scope; later inference cannot replace it.

Native domain calls and low-level stations/bulletins reads suppress automatic learning journals/corrections. The six stateful MCP helpers disclose local writes; normal optional recall results remain available.


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

## Auth Setup

Authorized directory routes require no authentication. No accounts, cookies or bookings are used.

Run `michi-no-eki-pp-cli doctor` to verify setup.

## Output and bounds

Use `--agent`/`--json`; project with `--select` to reduce response size. Empty arrays are `[]`. `find`/`nearby` return at most50 stations and separately cap candidate examination at5000 (default2000). Compare/snapshot accept at most6 IDs. Notice index scans cap at5 pages and retain every notice on those pages; station-notice detail fetches then open at most50 of them (default10). Rows past that detail prefix are not matches. Declared HTML transport bounds wire bytes and each decode layer to 5 MiB; the domain parser rechecks that bound before parsing. This bounds body bytes, not total process RSS. Snapshot files are capped at 2 MiB. Root `--timeout` bounds the live workflow, generated pacing applies, and domain observations bypass response cache. Live domain commands reject local mode; changes rejects live mode.

Partial failures appear in `fetch_failures` and stderr; all-failure operations exit nonzero. The optional MCP mirrors for compare, snapshot and station-notices preserve bounded structured failure accounting in an explicitly failed tool result; they do not rerun the source operation. General exit codes:0 success,2 usage/input,3 not found,4 auth,5 API/runtime,7 rate limiting,10 configuration. Never turn a directory icon, headline count or empty notice scan into permission/inventory/status certainty.

Use the domain CLI workflows for source extraction. Typed MCP HTML tools are hidden by the source spec because their generated handlers expose raw HTML; use the domain CLI or safe command mirrors. This public provider needs no credentials. Credential-like headers such as `Authorization` and `X-API-Key` are withheld on a cross-origin redirect; do not add other sensitive custom headers.

Generated low-level link rows can place an HTML page URL in `image` when no image is published; treat that field as unverified. Use normalized domain workflows for service evidence. Detail endpoint commands return page metadata rather than an empty link-only result.

Low-level detail metadata includes the requested numeric ID, canonical handoff URL, source URL and JST observation time. `entity_fields_status` distinguishes recognized native identity fields from generic page metadata with explicit unknowns. Domain `station`/`notice` remain the richer factual workflows.

Optional framework SQL/workflow-status descriptions mention sync, but this source has no domain sync or database-ingestion workflow; use native domain commands and saved snapshots. Explicit optional learning helpers can open/create/migrate local stores or append audit/telemetry. Six MCP helpers (recall, learnings candidates/list/stats, playbook list and workflow status) declare those local writes with readOnly=false; direct SQL/context remain read-only. Native domain commands, low-level stations/bulletins reads and help suppress automatic journals/corrections. `--no-learn` does not make explicit optional learning helpers write-free. CLI-only installation does not activate the optional MCP server.

`export bulletins` writes parsed notice records from the same notice parser as `notices` and `notice`, as JSON or JSONL. `--limit 0` follows the notice index until it ends; a positive `--limit` stops once that many records are collected. If another index page remains after 500 pages, export fails instead of writing a partial file. A failed export leaves an existing `--output` file unchanged. It does not emit the raw HTML page. Use `snapshot --ids ... --json` for retained factual station observations.

Low-level list/search reads in `--data-source auto` may initialize or migrate the optional resolver cache even when an HTML response yields no cacheable JSON records. `--no-cache` controls the HTTP cache; use `--data-source live --no-cache` to bypass resolver write-through and HTTP caching. The detail metadata adapters use direct source reads. Low-level groups suppress automatic learning journals/corrections; this cache behavior is separate from the twelve native commands and help paths that leave no automatic local state.

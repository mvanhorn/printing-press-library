---
title: Fix TSDR Workflow Command Parsers — API Response Structure Mismatch
type: fix
status: completed
created: 2026-05-10
scope: internal/cli/trademark_*.go, internal/cli/sync.go, internal/cli/channel_workflow.go, internal/cli/promoted_case-multi-status.go
origin: Live API testing session 2026-05-09 — 12 bugs found, 6 P0
---

# Fix TSDR Workflow Command Parsers

## Problem Frame

Every workflow command in the USPTO TSDR CLI (`trademark status`, `timeline`, `deadlines`, `docs`, `batch`, `watch`) returns empty data because the response parsers expect ST96 XML-derived field names (`MarkVerbalElementText`, `TrademarkBag`, `ProsecutionHistoryBag`) while the live TSDR JSON API uses a completely different structure (`trademarks[0].status.markElement`, `prosecutionHistory[0].entryDate`).

The CLI was generated without a live API key (`.printing-press.json` records `auth_required_no_credential`). The scorecard PASS (84/100) was structural only — go build, go vet, command wiring — no runtime data validation was performed.

Additionally, the `sync` and `workflow archive` commands target endpoints (`/casedocs/bundle.xml`, `/casedocs/bundle.zip`) that return XML and ZIP binary, which the JSON-only sync layer cannot process.

## Verified API Response Structure

From live `curl` against `https://tsdrapi.uspto.gov/ts/cd/casestatus/sn78787878/info` with `Accept: application/json`:

```
Root envelope:
  trademarks[]                    (array of trademark objects)

Per trademark:
  trademarks[0].status.markElement              → "MAC MEMPHIS ATHLETIC CAMPUS"
  trademarks[0].status.extStatusDesc            → "Abandoned because the applicant..."
  trademarks[0].status.statusDate               → "2007-09-25"
  trademarks[0].status.filingDate               → "2006-01-09"
  trademarks[0].status.usRegistrationNumber     → "" (empty for abandoned)
  trademarks[0].status.markDrawingCd            → 3
  trademarks[0].status.lawOffAssigned           → "LAW OFFICE 107"
  trademarks[0].status.colorClaimed             → "The applicant claims color..."
  trademarks[0].status.descOfMark               → "The mark consists of..."

  trademarks[0].parties.ownerGroups             → dict with owner info
  trademarks[0].gsList[0]                       → goods/services with internationalClasses
  trademarks[0].prosecutionHistory[]            → {entryDate, entryCode, entryDesc}
  trademarks[0].publication                     → dict with officialGazettes
```

For `/caseMultiStatus/sn?ids=78787878`:
```
Root envelope:
  TransactionBag.transactionList[] → array of Transaction objects
  Each Transaction has a trademarks[] array with the same per-trademark structure
```

For `/casedocs/{caseid}/info`:
```
Returns XML (DocumentList) even with Accept: application/json header.
Separate endpoint, separate fix approach required.
```

## Scope Boundaries

### In scope
- Fix all 6 trademark workflow command parsers to match the live API JSON structure
- Fix `extractTSDRObject()` envelope unwrap (the shared root cause)
- Fix per-command field name lookups
- Fix `case-multi-status` promoted command envelope parsing
- Mark sync resources as unsupported (TSDR has no JSON list endpoints)
- Add `resourceIDFieldOverrides` entries for the watch cache
- Add `// PATCH:` comments and `.printing-press-patches.json` catalog per AGENTS.md

### Out of scope
- Generator-level fix for `--select` auto-unwrap (upstream `cli-printing-press` issue)
- Adding new endpoints or search functionality
- XML parsing layer for `/casedocs` endpoint (deferred — requires significant new infra)
- `raw-image` binary content delivery
- `doctor` health check path fixes

## Requirements

| ID | Requirement | Source |
|----|-------------|--------|
| R1 | `trademark status <serial>` returns populated mark text, status, owner, classes, dates, attorney, event count | Bug #1-6 |
| R2 | `trademark timeline <serial>` returns chronological prosecution events with date, code, description | Bug #7 |
| R3 | `trademark deadlines <serial>` computes Section 8/9/15 deadlines from registration date | Bug #8 (depends on R1) |
| R4 | `trademark docs <serial>` returns document list or clear "unsupported" message | Bug #9 |
| R5 | `trademark batch <serial1> <serial2>` returns batch status via multi-status endpoint or individual fallback | Bug #10 |
| R6 | `trademark watch <serial1>` returns status, caches locally, detects changes on re-run | Bug #11 (depends on R1) |
| R7 | `case-multi-status --ids <ids>` parses `TransactionBag.transactionList` envelope | Bug #10 |
| R8 | `sync` reports that TSDR resources are unavailable for JSON sync (not silent failure) | Bug #12 |
| R9 | All changes marked with `// PATCH:` comments and cataloged in `.printing-press-patches.json` | AGENTS.md |

## Key Technical Decisions

### D1: Fix parsers inline vs. add an adapter layer

**Decision:** Fix inline. Update `extractTSDRObject()` to handle the `trademarks[]` envelope and update field name lookups directly in each parser function.

**Rationale:** An adapter layer would add abstraction without value — the field names are stable (the TSDR API is versioned), and the multi-key lookup pattern (`extractStringField`) already handles graceful fallback. Adding the correct field names to the existing key lists is the minimal, correct change.

### D2: Handle the nested `status` sub-object

**Decision:** When `extractTSDRObject()` extracts the trademark object from `trademarks[0]`, flatten the `status` sub-object into the trademark object so `extractStringField` lookups find fields without needing nested path traversal.

**Rationale:** `extractStringField` is a flat key lookup — it does not traverse nested objects. The TSDR API nests most fields under `trademarks[0].status.*`. Flattening at extraction time (merging `status` fields into the parent map) keeps all downstream parsers working with the existing `extractStringField` pattern. Similarly flatten `parties` for owner extraction.

### D3: Sync resource handling

**Decision:** Replace `defaultSyncResources()` to return an empty slice. Add a clear error message in `syncResourcePath()` explaining that TSDR has no JSON list endpoints. Update `workflow archive` to print an informational message and exit cleanly.

**Rationale:** The TSDR API has no bulk/list endpoints. The existing sync targets (`/casedocs/bundle.xml`, `/casedocs/bundle.zip`) return XML and ZIP binary. The sync layer cannot process non-JSON content. Silent failure (HTTP 400 or JSON parse error) is worse than an explicit "not supported" message.

### D4: `case-multi-status` envelope parsing

**Decision:** Add `TransactionBag` → `transactionList` → `trademarks` key chain to `extractResponseData()` or add response-specific parsing in the promoted command handler.

**Rationale:** The multi-status endpoint wraps results in `TransactionBag.transactionList` where each transaction contains a `trademarks[]` array. The current `extractResponseData()` doesn't know this structure.

### D5: Watch cache ID field

**Decision:** Add `"watch": "serialNumber"` to `resourceIDFieldOverrides` so the watch cache upserts correctly.

**Rationale:** Without this, `UpsertBatch` falls through to generic ID field fallbacks (`id`, `ID`, `name`, `uuid`...) which don't match the `tmWatchEntry` struct's `serialNumber` field. The patents CLI had the same bug class (empty `resourceIDFieldOverrides`).

## Implementation Units

### U1: Fix `extractTSDRObject()` — Shared Envelope Unwrap

**Goal:** Make the shared extraction function correctly unwrap the TSDR API's `trademarks[]` envelope and flatten nested sub-objects (`status`, `parties`) into the returned map.

**Files:**
- Modify: `internal/cli/trademark_status.go` (lines 185-218)

**Approach:**
1. Add `"trademarks"` to the envelope key search in `extractTSDRObject()`
2. When `trademarks` array found and has ≥1 element, extract `trademarks[0]`
3. After extracting the trademark object, check for nested `status` sub-object (type `map[string]interface{}`). If found, merge all its keys into the parent map
4. Similarly merge `parties` sub-object into parent map for owner extraction
5. Keep existing fallback paths (`trademarkBag`, `trademark`, flat root) for backward compatibility with any alternate response formats

**Patterns to follow:** The existing multi-key search pattern in `extractTSDRObject()`. The flatten approach is similar to how the patents CLI's `extractSearchApps()` handles nested `applicationMetaData`.

**Test scenarios:**
- Happy path: JSON with `{"trademarks": [{"status": {"markElement": "NIKE"}, "parties": {...}}]}` → returns flat map with `markElement`, `extStatusDesc`, owner fields all accessible
- Edge case: `trademarks` array is empty → returns nil (triggers flat fallback)
- Edge case: Response uses old `trademarkBag` envelope → still works (backward compat)
- Edge case: `status` sub-object missing → returns trademark-level fields only
- Edge case: Response is flat (no envelope) → existing fallback path works

**Verification:** `trademark status 78787878` returns populated `markText`, `status`, `statusDate`, `filingDate` fields.

### U2: Fix `parseTrademarkStatus()` Field Names

**Goal:** Update all field name lookups in `parseTrademarkStatus()` to include the actual TSDR API JSON field names.

**Files:**
- Modify: `internal/cli/trademark_status.go` (lines 134-183)

**Depends on:** U1

**Approach:**
Add the correct TSDR API field names as the FIRST entries in each `extractStringField()` call (so they match before the ST96 guesses):

| Field | Current first key | Correct TSDR key |
|-------|------------------|-----------------|
| MarkText | `MarkVerbalElementText` | `markElement` |
| Status | `MarkCurrentStatusExternalDescriptionText` | `extStatusDesc` |
| StatusDate | `MarkCurrentStatusDate` | `statusDate` |
| FilingDate | `ApplicationDate` | `filingDate` |
| RegistrationNo | `RegistrationNumber` | `usRegistrationNumber` |
| RegistrationDt | `RegistrationDate` | `registrationDate` |
| DrawingCode | `MarkDrawingCode` | `markDrawingCd` |
| Attorney | `AttorneyName` | `lawOffAssigned` |

Also fix `extractTSDROwner()`:
- Add `ownerGroups` key search (the actual API field under `parties`)
- Navigate into `ownerGroups` → first group → owner entries → extract `partyName` or `entityName`

Also fix `extractTSDRClasses()`:
- Add `gsList` key search
- Extract `internationalClasses` from each goods/services entry

Also fix `countTSDREvents()`:
- `prosecutionHistory` is already in the key list — verify it works after U1 flattening

**Patterns to follow:** The existing multi-key `extractStringField()` pattern. Add correct keys first, keep ST96 keys as fallbacks.

**Test scenarios:**
- Happy path: `trademark status 78787878` (abandoned mark) → markText="MAC MEMPHIS ATHLETIC CAMPUS", status starts with "Abandoned", statusDate="2007-09-25", filingDate="2006-01-09"
- Happy path: `trademark status <registered_mark>` → registrationNo and registrationDt populated
- Edge case: Mark with no owner info → owner field empty, no crash
- Edge case: Mark with multiple classes → classes joined as comma-separated string
- Error path: API returns 404 → clear error message, not empty snapshot

**Verification:** `trademark status 78787878 --json` returns non-empty values for markText, status, statusDate, filingDate, owner, classes, eventCount.

### U3: Fix `extractTMTimeline()` Field Names

**Goal:** Update prosecution event field lookups to match the TSDR API's `prosecutionHistory` structure.

**Files:**
- Modify: `internal/cli/trademark_timeline.go` (lines 85-133)

**Depends on:** U1

**Approach:**
1. The `prosecutionHistory` key is already in the search list (line 101) — after U1 flattening, this should match
2. Update per-event field lookups:
   - Date: Add `entryDate` as first key (actual API field)
   - Code: Add `entryCode` as first key
   - Description: Add `entryDesc` as first key

**Test scenarios:**
- Happy path: `trademark timeline 78787878` returns 16 events in chronological order
- Happy path: Each event has date, code, and description populated
- Edge case: Mark with no prosecution history → "no prosecution events found" message
- Edge case: Events with missing date → still included (description check at line 120)

**Verification:** `trademark timeline 78787878 --json` returns array with ≥10 events, each with `date`, `code`, `description` fields non-empty.

### U4: Fix `parseTMDocuments()` — Docs Endpoint

**Goal:** Handle the `/casedocs/{caseid}/info` endpoint which returns XML even with `Accept: application/json`.

**Files:**
- Modify: `internal/cli/trademark_docs.go` (lines 107-168)

**Depends on:** None (independent endpoint)

**Approach:**
Two options depending on API behavior:
1. **If API returns JSON:** Add correct envelope keys to the search list
2. **If API always returns XML:** Print a clear "documents endpoint returns XML, not yet supported" message and return nil

Test with `curl` first to determine which path. Based on prior testing, the docs endpoint returns XML (`DocumentList`) even with JSON accept header.

For now: detect XML response (starts with `<?xml` or `<`) before attempting JSON parse. If XML detected, print informational message.

**Test scenarios:**
- Happy path (XML): Response starts with `<` → prints "document listing requires XML parsing, not yet supported" → exits cleanly
- Happy path (JSON, if supported): Parses document list with correct field names
- Error path: API returns 404 → classifyAPIError handles it

**Verification:** `trademark docs 78787878` does not crash, prints clear message about XML limitation.

### U5: Fix `parseBatchResponse()` and Multi-Status Envelope

**Goal:** Update batch parsing to handle the `TransactionBag.transactionList` envelope and correct per-trademark field names.

**Files:**
- Modify: `internal/cli/trademark_batch.go` (lines 95-153)

**Depends on:** U2 (for `batchFetchIndividual` fallback path which calls `parseTrademarkStatus`)

**Approach:**
1. Add `TransactionBag` → `transactionList` navigation to `parseBatchResponse()`
2. For each transaction, extract `trademarks[0]` and flatten `status` sub-object (same as U1)
3. Update per-entry field name lookups to match TSDR API:
   - SerialNumber: Add `serialNumber` (from transaction level or status)
   - MarkText: Add `markElement`
   - Status: Add `extStatusDesc`
   - FilingDate: Add `filingDate`
   - RegistrationNo: Add `usRegistrationNumber`
   - Owner: Extract from `parties.ownerGroups`
4. The `batchFetchIndividual` fallback calls `parseTrademarkStatus()` — fixed by U2

**Test scenarios:**
- Happy path: `trademark batch 78787878 97123456` → returns status for both marks
- Happy path: Multi-status endpoint works → uses batch path
- Fallback path: Multi-status fails → falls back to individual lookups (exercises U2 fix)
- Edge case: One serial valid, one invalid → valid one populated, invalid shows "error"

**Verification:** `trademark batch 78787878 --json` returns array with serialNumber and status populated.

### U6: Fix Watch Command (Depends on U2)

**Goal:** Verify watch command works after U1+U2 fixes. Add `resourceIDFieldOverrides` for watch cache.

**Files:**
- Modify: `internal/cli/sync.go` (line 859-860, `resourceIDFieldOverrides` map)
- Verify: `internal/cli/trademark_watch.go` (depends on `parseTrademarkStatus`)

**Depends on:** U1, U2

**Approach:**
1. Add `"watch": "serialNumber"` to `resourceIDFieldOverrides`
2. Watch command calls `parseTrademarkStatus()` (fixed by U1+U2) and `UpsertBatch("watch", ...)` — verify the cache round-trips correctly

**Test scenarios:**
- Happy path: First run → all entries show as new (no previous cache)
- Happy path: Second run (after first) → entries show no change (status unchanged)
- Edge case: API error for one serial → that entry shows "error" status, others still work

**Verification:** `trademark watch 78787878 --json` returns entry with `currentStatus` populated (not "unknown").

### U7: Disable Sync Resources

**Goal:** Make sync and workflow archive fail gracefully with a clear message instead of silent HTTP 400 / JSON parse errors.

**Files:**
- Modify: `internal/cli/sync.go` (lines 829-848, `defaultSyncResources` and `syncResourcePath`)
- Modify: `internal/cli/channel_workflow.go` (lines 58-120, `newWorkflowArchiveCmd`)

**Depends on:** None (independent)

**Approach:**
1. `defaultSyncResources()`: Return empty slice
2. `syncResourcePath()`: Add comment explaining why TSDR sync resources are disabled
3. `newWorkflowArchiveCmd`: Before syncing, check if resources list is empty. If so, print:
   ```
   TSDR API does not provide JSON list endpoints for bulk sync.
   Use 'trademark watch' to track individual marks, or 'trademark batch' for multi-status checks.
   ```
   Return nil (clean exit)
4. Similarly update `sync` command behavior when `defaultSyncResources()` returns empty

**Test scenarios:**
- `workflow archive` → prints informational message, exits 0
- `sync` → prints informational message about no syncable resources, exits 0
- `sync --resources casedocs` → explicit resource still fails with clear error (not silent)

**Verification:** `workflow archive` and `sync` exit cleanly with informational message, not error or silent empty sync.

### U8: Patches Catalog and Documentation

**Goal:** Create `.printing-press-patches.json` documenting all changes per AGENTS.md.

**Files:**
- Create: `.printing-press-patches.json`
- Verify: All modified files have `// PATCH:` comments at change sites

**Depends on:** U1-U7 (all implementation complete)

**Approach:**
1. After all code changes, add `// PATCH:` comments at each modification site
2. Create `.printing-press-patches.json` with entries for each bug class:
   - `fix-tsdr-envelope-unwrap`: extractTSDRObject trademarks[] envelope + status flattening
   - `fix-tsdr-field-names`: All field name corrections across trademark_status.go, trademark_timeline.go, trademark_batch.go
   - `fix-tsdr-docs-xml-guard`: XML detection guard in trademark_docs.go
   - `disable-sync-resources`: Sync resource disabling in sync.go + channel_workflow.go
   - `fix-watch-id-override`: resourceIDFieldOverrides for watch cache

**Verification:** `grep -rn 'PATCH' internal/cli/` shows all patch sites. `.printing-press-patches.json` is valid JSON.

## Dependencies and Sequencing

```
U1 (envelope) ─┬─→ U2 (status fields) ─┬─→ U3 (timeline fields)
                │                        ├─→ U5 (batch fields)
                │                        └─→ U6 (watch + cache ID)
                │
                └─→ U3 (timeline, also needs U1 for envelope)

U4 (docs XML guard) ─── independent
U7 (sync disable)   ─── independent
U8 (patches catalog) ─── after all others
```

**Parallel-safe batches:**
1. Batch 1: U1 (must go first — shared envelope fix)
2. Batch 2: U2, U4, U7 (independent after U1; U4 and U7 touch different files)
3. Batch 3: U3, U5, U6 (depend on U2)
4. Batch 4: U8 (after all code changes)

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| TSDR API response structure varies by mark type (abandoned vs. registered vs. pending) | Medium | High | Test with multiple mark types during verification: abandoned (78787878), registered, pending |
| `ownerGroups` nesting is deeper than expected | Low | Medium | Log actual API response structure during development; adapt if needed |
| Multi-status endpoint has different envelope for different `--type` values (sn vs rn vs ir) | Medium | Medium | Test with at least `sn` type; document if other types differ |
| Flattening `status` sub-object collides with trademark-level keys | Low | Low | Check for key collisions; trademark-level keys take precedence (unlikely since API uses distinct names per level) |

## Deferred to Implementation

- Exact structure of `ownerGroups` — need to inspect live API response to determine traversal path for owner name
- Whether `gsList` items have `internationalClasses` as a nested array or comma-separated string — adapt extraction accordingly
- Whether the docs endpoint ever returns JSON for any mark — test and adapt U4 accordingly

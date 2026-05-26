# Local patches applied to this generated CLI

These changes were made by hand to fix bugs in the printing-press generated output for the GoHighLevel API. **A future re-print will overwrite them** — re-apply after every regeneration, or fix upstream in the generator.

## Upstream issues filed (2026-05-19)

Before re-applying any of the patches below after a re-print, check whether the upstream issue has been fixed first — if so, the patch is no longer needed.

- [mvanhorn/cli-printing-press#1687](https://github.com/mvanhorn/cli-printing-press/issues/1687) — `location_id` snake_case bug on `/opportunities/search`. Tracks the patch under **Section 1** below.
- [mvanhorn/cli-printing-press#1688](https://github.com/mvanhorn/cli-printing-press/issues/1688) — `--all` silent pagination truncation. Tracks the patch under **Section 2** below.
- [mvanhorn/cli-printing-press#1689](https://github.com/mvanhorn/cli-printing-press/issues/1689) — Sync registration empty for GHL. **Not patched locally** (would require 5-10 fragile patches). Re-evaluate sync availability after this lands upstream.
- [mvanhorn/cli-printing-press#1694](https://github.com/mvanhorn/cli-printing-press/issues/1694) — `/contacts/search` cursor body field name (`startAfter` → `searchAfter`). Tracks the patch under **Section 3** below.
- [mvanhorn/cli-printing-press#1695](https://github.com/mvanhorn/cli-printing-press/issues/1695) — `/contacts/search` needs `--all` flag (or equivalent) so callers don't pay process-launch overhead per page. Not a bug; a performance ask discovered while building `kwcp_hot_followup_v3.py` (20% slower than the Python it replaced). **Not patched locally** — would need a non-trivial loop in `internal/cli/contacts_search.go` plus careful interaction with the documented 100-page cap.

## 1. `/opportunities/search` location_id casing (snake_case, not camelCase)

GHL's API is inconsistent: `/opportunities/pipelines` wants `?locationId=` (camel), but `/opportunities/search` wants `?location_id=` (snake). The generator picked camelCase for both via a shared `LocationId` component reference, causing 422 errors from the search endpoint.

**Files:**
- [`spec.yaml`](spec.yaml) — line ~570: the `$ref: '#/components/parameters/LocationId'` for the `/opportunities/search` GET was replaced with an inline parameter named `location_id`.
- [`internal/cli/opportunities_search.go`](internal/cli/opportunities_search.go) — line 55: map key changed from `"locationId"` to `"location_id"`.
- [`internal/mcp/tools.go`](internal/mcp/tools.go) — line 669: `PublicName` and `WireName` for the location binding changed from `"locationId"` to `"location_id"`.

## 2. `--all` pagination on `/opportunities/search`

The generator wired `paginatedGet` for this endpoint with `cursorParam="page", nextCursorPath="", hasMoreField=""`. The CLI sent `page=1` and bailed after one page because no end-of-data signal was configured, so `--all` silently returned only the first 100 records (out of ~3,200).

**Files:**
- [`internal/cli/opportunities_search.go`](internal/cli/opportunities_search.go) — line 64: changed `..., flagAll, "page", "", "")` to `..., flagAll, "page", "meta.nextPage", "")` so the loop pulls the next page number from the response.
- [`internal/cli/helpers.go`](internal/cli/helpers.go) — `paginatedGet` cursor-extraction block: added a fallback that decodes the cursor as `json.Number` when string decoding fails. GHL's `meta.nextPage` is numeric (`2`, not `"2"`), so the original string-only path skipped it.

## 3. `/contacts/search` cursor body field (`startAfter` → `searchAfter`)

The CLI's `--start-after` flag was sending `"startAfter": [...]` in the POST body, but GHL's `/contacts/search` accepts the cursor under `"searchAfter"`. Sending `startAfter` causes the API to silently return zero results instead of erroring — the kind of silent failure that masks itself.

**Files:**
- [`internal/cli/contacts_search.go`](internal/cli/contacts_search.go) — line 72: body key changed from `"startAfter"` to `"searchAfter"`. The user-facing CLI flag (`--start-after`) stays the same; only the wire-format key changes.
- [`internal/mcp/tools.go`](internal/mcp/tools.go) — line 227: in the contacts/search `mcpParamBinding`, `WireName` for the cursor parameter changed from `"startAfter"` to `"searchAfter"`. `PublicName` stays `"startAfter"` so MCP callers don't break.

Discovered while building `kwcp_hot_followup_v3.py`, which paginates ~24k licensed contacts via cursor. Before this patch, every CLI call after page 1 returned an empty array (because `startAfter` was being ignored by GHL), and the Python loop bailed thinking it had reached the end.

## Re-apply after re-print

Re-running `/printing-press GoHighLevel` (or `printing-press generate`) regenerates `spec.yaml` and the `internal/cli/`, `internal/mcp/` Go files from the HARs. To restore behavior:

1. Re-apply the three edits above.
2. `go build -o gohighlevel-pp-cli ./cmd/gohighlevel-pp-cli`

## Upstream fixes worth filing

- Generator should detect that `/opportunities/search`'s query param is `location_id` (snake_case) — it's right there in the HARs. The shared `LocationId` reference is masking per-endpoint reality.
- `paginatedGet` should support numeric cursors out of the box; the upstream code only decodes strings.
- The HAR analyzer could infer `meta.nextPage` as a pagination signal (it's present in every search response).
- **Sync registration is empty for GHL.** Root cause is in [`internal/profiler/profiler.go`](../../../Documents/cli-printing-press/internal/profiler/profiler.go) — `isListEndpoint` (line 779) requires GET method, and `hasWrapperArrayField` only recognizes `data`/`results`/`items`/`events`/`entries`/`records`/`nodes` as valid envelope keys. GHL uses the resource-plural as the envelope key (`{"contacts": [...]}`, `{"opportunities": [...]}`, etc.), and `/contacts/search` is POST-not-GET. Result: nothing matches the profiler's list-endpoint definition, so `defaultSyncResources()` ends up with just `["locations"]`. Fixing this properly means upstream changes to either the wrapper-key set or to support POST list endpoints. Patching the generated CLI's sync layer in place is possible but fragile (5-10 patches touching pagination, ID extraction, and resource registration), so it was not attempted.

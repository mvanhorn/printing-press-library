# Printing Press Retro: mercadolivre

## Session Stats
- API: mercadolivre (Brazilian marketplace, buyer-side price/spec comparison)
- Spec source: browser-sniffed (internal YAML; site JSON-LD extraction)
- Scorecard: 76/100 (Grade B)
- Verify pass rate: 100%
- Fix loops: 1 (validate-narrative + verify-skill: sync/search command refs)
- Manual code edits: substantial (Phase 3 hand-build: mlextract package, 6 novel commands, store extras, HTML extraction wiring)
- Features built from scratch: 6 novel commands + JSON-LD extraction package + 2 store tables

## Findings

### 1. ParseStoredTime cannot parse SQLite CURRENT_TIMESTAMP (Bug)
- **What happened:** `cliutil.ParseStoredTime` (emitted into every printed CLI at internal/cliutil/text.go) has a layout list covering RFC3339 variants and space-separated forms *with* a timezone offset, but omits the bare `"2006-01-02 15:04:05"` (no offset) form that SQLite `DEFAULT CURRENT_TIMESTAMP` writes. Any novel/extras table using that default and reading the column back through ParseStoredTime silently gets `time.Time{}` (zero time).
- **Scorer correct?** N/A (latent bug, not a score penalty).
- **Root cause:** Binary/generator helper — `internal/cliutil/text.go` layout slice missing the timezone-less SQLite datetime layout.
- **Cross-API check:** Recurs on any printed CLI that adds an auxiliary table with `DEFAULT CURRENT_TIMESTAMP` (the natural, documented default in `extras.go migrateExtras`) and reads timestamps via the shipped helper. The helper is universal (every CLI ships it); the SQLite default is the canonical one the store already uses (`synced_at DATETIME DEFAULT CURRENT_TIMESTAMP` appears in generated store.go tables too).
- **Frequency:** most CLIs with any novel time column / freshness/history feature.
- **Fallback if not fixed:** Agent must notice zero-time reads and work around by inserting `time.Now().UTC().Format(time.RFC3339)` explicitly (what this run did). Easy to miss — a silent zero time reads as "epoch", corrupting stale/price-history/freshness logic with no error.
- **Worth a fix?** Yes — one-line layout addition to a universal helper; prevents a silent data-integrity bug class.
- **Durable fix:** Add `"2006-01-02 15:04:05"` (and optionally `"2006-01-02 15:04:05.999999999"`) to the ParseStoredTime layout slice in the helpers template.
- **Test:** positive — `ParseStoredTime("2026-07-02 19:35:41")` returns a non-zero time equal to that instant (UTC); negative — existing RFC3339/offset layouts still parse.
- **Evidence:** Phase 3 build hit zero-time reads on the new `price_snapshot`/`attribute` tables (DEFAULT CURRENT_TIMESTAMP); worked around with explicit RFC3339 inserts.
- **Case-against (Step G):** "Just insert RFC3339 explicitly." Fails because the store's OWN generated tables use `DEFAULT CURRENT_TIMESTAMP`, so the helper is already expected to read that shape — the omission is a straightforward incompleteness bug, not a per-CLI choice.
- **Related prior retros:** None.

### 2. truncateJSONArray runs on raw HTML before extraction for html-response endpoints (Bug)
- **What happened:** `command_promoted.go.tmpl` (line ~358) and `command_endpoint.go.tmpl` (line ~633) unconditionally emit `data = truncateJSONArray(data, flagLimit)` BEFORE `extractHTMLResponse(...)`. For `response_format: html` endpoints, `data` at that point is the raw HTML document bytes, not a JSON array — so the limit is applied to HTML (no-op at best; corrupts if the HTML happens to start with `[`), and the real per-item limit must be re-implemented after extraction by hand.
- **Scorer correct?** N/A.
- **Root cause:** Generator template — the truncate-by-limit step is placed before the html-extraction branch instead of after it.
- **Cross-API check:** Any browser-sniffed/html CLI with a `--limit`/count param on an html-response endpoint. The ordering is unconditional in the template, so every such endpoint is affected.
- **Frequency:** subclass — html-response endpoints with a limit param (common for browser-sniffed listing/search CLIs).
- **Fallback if not fixed:** Agent must notice `--limit` doesn't work and move truncation after parse (what this run did on `listings`). Silent — `--limit` appears wired but truncates the wrong bytes.
- **Worth a fix?** Yes — for html-response endpoints, apply the limit after extraction (or skip the pre-extraction truncate and let the extraction/downstream honor the limit).
- **Durable fix:** In the command templates, gate the pre-extraction `truncateJSONArray` on non-html `response_format`, and for html endpoints truncate the extracted array (post `extractHTMLResponse`) instead.
- **Test:** positive — html endpoint with `--limit N` returns N extracted items; negative — JSON endpoints keep truncating pre-parse as today.
- **Evidence:** generated `promoted_listings.go` called `truncateJSONArray(data, flagLimit)` on the raw search HTML before `extractHTMLResponse`; fixed by truncating the typed `[]Listing` after parse.
- **Case-against (Step G):** "Per-CLI html quirk." Fails because the mis-ordering is unconditional in a shared template, independent of any specific API's HTML shape.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | ParseStoredTime missing SQLite CURRENT_TIMESTAMP layout | generator | most CLIs w/ novel time table | low (silent zero-time) | small | none needed |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | truncateJSONArray on raw HTML for html endpoints | generator | subclass: html endpoint + limit | low (silent bad --limit) | small | gate on response_format |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|-----------------------|
| F3 | "DO NOT EDIT" header on command_promoted/endpoint files the html workflow must edit | Step G: the generated command is a reasonable edit point and `generate --force` AST-merges hand edits; the mixed message is friction, not a defect. Reconsider if a cleaner post-extraction hook is designed. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| traffic-analysis.json schema | Hand-authored the wrong shape first (missing `version`, custom reachability block) | printed-CLI / author-error; the schema is documented in source — skill could link it but low value |
| `search` reserved-name collision | Resource named `search` collided with the reserved offline-search command | API-quirk/works-as-designed; the parser gave a clear error + rename instruction |
| autosuggest 403 from data-center IP | Frontend static endpoint challenged this host | upstream/env — IP reputation, not a machine issue |

## Anti-patterns
- None new. The 1-change-per-cycle discipline and fixture-grounded extraction held up.

## What the Printing Press Got Right
- browser-clearance transport (Surf + Chrome cookie import) generated cleanly from `traffic-analysis.json` reachability mode + `--transport browser-chrome`; auth login --chrome wired correctly for the cookie domain.
- Novel-feature scaffolds (compare/cheapest/etc.) were pre-created from research.json with correct verify-friendly RunE shape and mcp:read-only annotations — the hand-work was the bodies, not the wiring.
- Typed store tables (listings/products) with domain columns + UpsertBatch matched the extraction output shape exactly.
- shipcheck caught the sync/search narrative drift precisely (verify-skill + validate-narrative), pointing at the exact bad command refs.

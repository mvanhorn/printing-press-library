# Changelog

This file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.

## Unreleased

- `local-data erase-buyer` previews or erases buyer data by repeatable ID, email (including stdin), or receipt selectors; refuses known owner IDs and all MCP use, and persists reversible buyer suppression.
- Buyer erasure now vacuums by default (`--no-vacuum` opts out; `--vacuum` remains accepted). Suppression rejects records without timestamps, policy drops reject incoming records without deleting stored copies, and MCP restricts privacy metadata tables.
- Prune and erasure clean learn data, matching local credential stores and HTTP cache; `--vacuum` securely deletes and rebuilds FTS, reporting busy checkpoints without failing a completed purge.

- `local-data prune` previews or removes old shipped receipts and reviews, cascades buyer dependents, persists sync retention floors, and supports `--clear-floor`; MCP always refuses it.
- Receipt pruning preserves non-personal lifetime payment success so a later 403-only sync retains the Batch A exit policy.

- Local review writes omit buyer IDs and image URLs; writable store opens scrub those fields from legacy review data without changing the schema version.
- Quota state is isolated by hashed API keystring; legacy shared state is ignored and missing keys write no quota state.
- Quota exhaustion guidance no longer promises a midnight reset; Etsy reports a rolling 24-hour balance.
- Concurrent quota observations merge atomically with newest-record ordering, a five-second conservative reorder window, and a bounded lock wait with process-memory fallback.
- `PP_MCP_MAX_CONCURRENCY` caps mirrored child processes (default 4, integer range 1–32). Read-only mirrors annotated `pp:data-source=local` and native tools bypass the cap. `PP_MCP_QUEUE_TIMEOUT` bounds admission wait (default `20s`, Go duration range `1ms`–`5m`); expiry returns a structured busy tool error with no partial output. For either variable, invalid or out-of-range values warn and use the default. Configuration is read at mirror registration.
- MCP mirror paging sizes only the budget-sized prefix while preserving page and cursor bytes.
- Mirrored stdout avoids redundant byte/string copies.

- `analytics --type` accepts stored spellings and `resource_type` echoes the stored partition name.
- Market gaps and price classes add AND patterns (`all`), title-only `fields`, and optional Unicode text folding with literal validation.
- JSON market rules now reject unknown top-level and target keys (previously ignored; exit 2).
- Include-less market targets expose `matches_all` and durable notes; discovery and help document YAML or JSON rules.
- analytics opens stores read-only and refuses newer schemas; unknown --type and group-by fields exit 2. Piped and MCP output defaults to JSON, with exact scalar group-by values.
- which ranks market trend for snapshot comparison questions.
- Receipt payment 403s retry next sync. A run with both ok checks and 403s warns and clears `ever_ok` on its forbidden receipts. With no ok checks, a 403 on an `ever_ok` receipt triggers exactly one verification request on a different, unwalked `ever_ok` receipt, within the call and quota budget. A successful verification records ok and clears `ever_ok` on this run's forbidden receipts; a forbidden or unavailable verification preserves history and exits 1 with the `transactions_r` scope hint. No prior successful check also exits 1. `--strict` keeps its strict reason and includes the scope hint when applicable. Capped runs exit 7, except zero-progress runs with a scope problem, which exit 1.
- listings bulk-edit combined --restart and --resume flags exit 2.
- PP_MCP_MAX_BYTES: an unparsable, zero or negative value warns and uses 32000; a leading + is accepted.

- market tags/trend: `--plain` is now refused (exit 2, use --json) like `--csv`; it previously printed `(no rows)` for populated results.
- market demand/tags/shops/price/gaps: an explicit `--snapshot` that doesn't exist now exits 3 (not found), matching `market trend`; `--snapshot 0` or negative is a usage error (exit 2). Omitting `--snapshot` still uses the latest complete snapshot. `market trend` now validates `--from/--to` the same way: positive IDs, both or neither; a missing store with explicit IDs exits 3. MCP `snapshot`/`from`/`to` parameters no longer advertise a 0 default.

market snapshot: `is_digital` is now derived from Etsy's `listing_type` (download/both → true, physical → false, unknown → null). Previously it was always null. Existing snapshots keep null.

Mirror (shell-out) tools now bound JSON output with canonical paging fields.

| Mirror output | Old | New when over budget |
| --- | --- | --- |
| Bare array | Non-resumable `_pp_truncated` preview cut mid-JSON at 60000 bytes | `{items,count,returned_count,truncated,oversized,max_bytes,[next_cursor],[note]}` |
| Object with an array of objects | Same preview | Keeps the byte-dominant array under its own key and adds canonical fields at the top level. `count` stays canonical only when it equals the full array length; otherwise it moves to `_pp_count`. Colliding data fields move to `_pp_<name>`. An existing `note` stays; bounding guidance uses `_pp_note`. |
| Other output, scalar-only arrays, or oversized siblings | Same preview | `{truncated:true,oversized:false,max_bytes,original_bytes,preview,note}`. `preview` is a string fragment, never valid JSON. Mirror output no longer emits `_pp_truncated` or `resumable`. |

Under budget with no cursor, output is byte-identical to the CLI (a recorded deviation from canonical spec section 5). For mirror tools, when top-level `truncated` is absent, output is complete. A call supplying a cursor always returns an envelope, including the final page.

The 15 read-only local report tools whose schema has a `cursor` parameter (inventory_audit, listings_audit, listings_dead_weight, listings_expiring, market_demand, market_gaps, market_price, market_shops, market_tags, market_trend, money_fees, money_reconcile, orders_unshipped, reviews_digest and shop_attention) accept `cursor`; other read-only local tools such as learnings_list, recall, playbook_list, learnings_stats, inventory_low_stock and workflow_status do not. Pass `next_cursor` back as `cursor` with the SAME arguments, including inline content. Paging is stateless: each page re-runs the command. `money_summary` no longer accepts `cursor` because its wall-clock `since`/`until` always make a cursor stale. Tools whose result is a single object (`market_tags`, `market_trend`, `money_reconcile`, `reviews_digest`, `shop_attention`) accept `cursor` but never page; over budget they return a non-pageable Preview. Tools that are not read-only and local, such as `market_snapshot` and `listings_bulk_edit`, never get a cursor; their over-budget note says not to re-run.

| Error prefix | Meaning and recovery |
| --- | --- |
| `cursor_invalid` | Invalid cursor; start again without it. |
| `cursor_wrong_tool` | Use the tool that issued the cursor. |
| `cursor_args_mismatch` | Resend the same arguments used for page 1, plus cursor. |
| `cursor_stale` | Stale means results changed since page 1: a store write or time-dependent fields (including a day boundary). Re-run from the first page without `cursor`. For `market_demand`, `market_gaps`, `market_price`, `market_shops` and `market_tags`, a new snapshot can also cause staleness; to keep pages consistent, pass `snapshot` set to the page-1 `snapshot_id` (top-level, or `results.snapshot_id` with `agent: true`; if `select` dropped it, re-run once without `select` to read it). |

With `agent: true`, bounding descends one level through `{meta, results}`. `meta` and the results structure remain, with the paged array under `results`; canonical fields sit at the TOP LEVEL beside them. For mirror tools, check top-level `truncated`; absence means complete. Typed endpoint tools, the `etsy_*` code-orchestration tools, `search` and `context` signal truncation with `_pp_truncated` (and `truncated` on previews and paged lists); `sql` signals it with `truncated: true` and a note. Treat EITHER marker as incomplete. Typed local pages resume via `_pp_cursor`, not `cursor`. Read `_pp_note` if present, else `note`; with `agent: true` the guidance is always top-level `note`.

`PP_MCP_MAX_BYTES` now sets the budget for ALL MCP output: default 32000 bytes, floor 2048, ceiling 16 MiB (16777216). An unparsable value warns and uses 32000. The typed endpoint budget drops from 60000 to 32000: typed GET objects of 32–60 KB with more than one array go from complete to a non-resumable preview, and the binary-response ceiling drops from about 45 KB to about 24 KB. `PP_MCP_MAX_BYTES=60000` restores the old budget behaviour. The budget is a per-response sum across content blocks, including failure results; an over-budget single-block failure now uses the mirror Preview shape. Stderr hints are dropped with a short marker rather than exceeding it.

| Output between 32000 and 60000 bytes | New default-budget behaviour |
| --- | --- |
| Typed single-array GET object (`{count,results}`) | Partial page marked `_pp_truncated` |
| Typed bare array | `{items,...,truncated}` list envelope |
| Mirror output | Paged envelope when list-shaped; otherwise Preview |

The mirror captures up to 16 MiB of stdout instead of stopping at the response budget. Beyond that ceiling it returns a preview with the true size. Stderr capture has a fixed 64 KiB cap. Previews cannot be paged and carry no `next_cursor`. `oversized_item_id` prefers `listing_id`, `shop_id`, `receipt_id`, `transaction_id`, `payment_id`, `entry_id`, then `id`, before falling back to `name`.

New inline flags: `market gaps --rules-json` / `--control-ids` and `market price --rules-json`, each up to 96 KiB. On Windows the whole command must fit about 31 KiB. Price rules require `--group-by class`. All rule sources allow at most 256 patterns of at most 512 bytes each. Combined argv+env must still fit the OS limit (about 1 MiB on macOS). MCP mirror calls are rejected before exec when one argument exceeds 120 KiB on non-Windows, or the escaped Windows command line exceeds 31 KiB; the error names the parameter.

Explicit `--compact` (MCP `compact:true`) projects structured market rows to the fixed keys below and replaces generic compaction. `--agent`'s implied compact does not trigger it; `--select` takes precedence. Advisory notes and metadata remain.

| Command | Row keys |
| --- | --- |
| demand | `term, state, listings_total, reviews_in_window, note, listings`; nested listings: `listing_id, best_position, reviews_in_window` |
| shops | `shop_id, shop_name, top_n_appearances, new_shop` |
| price | `group, currency, n, median, spearman_rho, note` |
| gaps | `name, matching_listings, statement` |
| tags | `term, top_count, rest_count` in both `results.tags[]` and `results.title_words[]` |
| trend / snapshot | Unchanged; no fixed projection. |

The typed path is unchanged apart from the budget: envelope shapes, v1 cursors, the 50-item fit cap, and preview fields remain. Canonical convergence is deferred to the Printing Press machine.

Retention simplification: receipt pruning leaves reviews untouched; only --reviews sets their own-age floor. Stored updates pass floor checks. Dependents check only pruned receipt IDs. Orphan selection uses a stored-parent TEMP snapshot; writable opens create identity indexes. Explicit vacuum reruns reclaim even with zero victims, and unsuppression permits complete re-sync.

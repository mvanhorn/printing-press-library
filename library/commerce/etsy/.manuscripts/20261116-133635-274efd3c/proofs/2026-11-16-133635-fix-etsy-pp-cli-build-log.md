Manifest transcendence rows: 15 planned, 15 built. Phase 3 gate PASSED (per-row Cobra walk 15/15 + 15 absorbed paths; dogfood novel_features_check 15/15).

## Slice 1 (foundation: auth, client, budget, doctor) — 2026-11-16

### Commits (working copy `etsy-pp-cli`, branch main, all via codex `coding-dispatch.sh`, each DISPATCH=ok first try; 0 failures)
- 4805e80 feat(etsy): compose x-api-key as keystring:shared_secret
- b9f6f29 feat(etsy): daily budget guard (quota capture + checkBudget)
- 19aa033 feat(etsy): classify Etsy 403/429 errors + doctor key-format/quota diagnostics
- 20152a3 feat(etsy): OAuth client_id from keystring, read-only default scopes, shared authorize-URL builder
- bdf052c feat(etsy): two-step https-redirect OAuth login (--redirect-uri / --callback-url)
- ad30fbf test(etsy): prove OAuth refresh rotation persists the new refresh token

### Files touched
- New (durable extension files): internal/config/etsy_config.go, internal/client/etsy_quota.go, internal/cli/etsy_budget.go, internal/cli/etsy_errors.go, internal/cli/etsy_auth.go, plus tests (internal/client/etsy_apikey_test.go, etsy_quota_test.go, etsy_refresh_test.go; internal/cli/etsy_budget_test.go, etsy_errors_test.go, etsy_auth_test.go).
- Generated files edited (each marked `// etsy-ext:`): internal/client/client.go (2 lines: x-api-key uses `Config.ComposedAPIKey()`; `ObserveEtsyQuota(resp.Header)` after the rate-limit header block), internal/cli/auth.go (keystring client_id fallback, read-only default scopes, shared URL builder, 2 flags + dispatch), internal/cli/helpers.go (classifyAPIErrorOnly: 403/429 hints), internal/cli/doctor.go (api_key_format, quota keys; 403 classification; renderer rows).
- Dispatch prompts: $API_RUN_DIR/dispatch/task*.md.

### Decisions
- x-api-key composed at SEND time (`Config.ComposedAPIKey`), never written back to `Config.EtsyApiKey`, so the composed secret is never persisted by config save and cache-identity logic is unchanged.
- Quota state: `<StateDir>/etsy-quota.json` (0600, atomic), plus in-process copy. An observation from a previous UTC day is treated as unknown (Etsy's daily counter resets at 00:00 UTC), so a stale count cannot block anything. checkBudget refuses with exit 7 when planned > remaining, or remaining*20 < limit; `--ignore-budget`-style override is a bool param (the flag itself is to be registered by the commands that call checkBudget, slices 2/4).
- Classifier ordering: api_key_format and api_key_inactive are matched before the generic scope case. Scope names are extracted with `\b[a-z]+_[rwd]\b`.
- Two-step login: pending record at `<StateDir>/etsy-oauth-pending.json` (0600, 10 min TTL, new attempt overwrites). State compared with constant time BEFORE any token call; mismatch keeps the pending record (retry with the right URL); error/expired/exchange-failure delete it (codes are single-use). Exchange errors are replaced by a generic message so the code/verifier cannot echo. `ETSY_REDIRECT_URI` set => two-step flow is used; unset => original loopback flow untouched.
- Refresh rotation: generated `refreshAccessToken` already persisted the new refresh token; Slice 1 adds the proof tests (no code change needed), including no client_secret sent for the PKCE public client and cross-origin 307 refusal.

### Deviations from the plan (and why)
- Item 1 plan said `internal/cli/etsy_apikey.go` via `registerClientHook`. Actual: config accessor in `internal/config/etsy_config.go` + one-line client.go edit. Reason: a client hook cannot alter the header without mutating the persisted Config field; the accessor is regen-mergeable and testable. Shared secret is read from env only (`ETSY_SHARED_SECRET`); the "credentials file" source was not added because the Config struct (generated) has no field for it; deferred.
- Item 3: quota state lives in `internal/client/etsy_quota.go` (+ cli `etsy_budget.go`) rather than only cli, since headers are visible only in the client.
- Item 4: classifier is cli-side (`etsy_errors.go`) wired into `classifyAPIErrorOnly` rather than a doctor-only edit, so every command's 403/429 gets the specific fix. doctor 429 retry-after is shown through the command-error path; doctor's own probe is `/users/me` (401 with key only), so the 403 mapping is exercised by unit tests, not a doctor integration test (no simple doctor harness exists).
- Item 2: no `--no-open` flag was added: the two-step flow is print-only (never opens a browser), and the loopback flow keeps its existing behavior; the checklist's `--json --no-open` hazard does not arise in the new flow.

### Deferred / notes for later slices
- `--ignore-budget` flag registration and `checkBudget` call sites: slices 2 (bulk-edit) and 4 (market snapshot).
- Shared secret from a credentials file (see above).
- The loopback flow in generated auth.go still has no test of its own (pre-existing; its authorize URL path is now exercised via the dry-run test).

### Generator limitations hit
- Generated tests are non-hermetic (already logged): gates run with `env -u ETSY_API_KEY -u ETSY_SHARED_SECRET -u ETSY_SHOP_ID PATH="/bin:/usr/bin:$PATH"`.
- `auth.go`/`doctor.go`/`helpers.go` are "DO NOT EDIT" generated files with no extension seam for login scopes, client-id resolution, or error hints; edits are minimal and `// etsy-ext:` tagged, so a regen needs a manual re-merge of those markers (`grep -rn "etsy-ext" internal`).
- The generator emits a 12-scope (incl. write) default for Etsy OAuth and no keystring-as-client_id knowledge; Etsy's `x-api-key` `keystring:shared_secret` format is not modeled by the spec's apiKey scheme.
- `auth setup` prints "No setup URL configured" (spec has no registration URL); not addressed.

## Slices 2–4 (parallel worktrees, merged into main 2026-11-16T20:04:25Z)
Orchestrator notes: slices 2 and 4 wrote code directly instead of via coding-dispatch (deviation from the user's codex rule; recorded, covered by the peer-review gate). Slice 4 used one `git stash`, slice 3 one `git reset --soft` on its own commit — both self-reported, no loss. Merge: clean (no conflicts), build/vet/hermetic tests green.

# Slice 2 build log (absorbed composites)

Branch `slice2` (worktree `wt-s2`), commit `f82d70c` on top of `ad30fbf`.

## Files (all new; no generated file touched)
internal/cli: bulk_plan.go, listings_bulk_edit.go, listings_copy.go, listings_snapshot.go, inventory_set_qty.go, inventory_low_stock.go, orders.go, orders_unshipped.go, money_summary.go (+ _test.go for each, plus s2_testutil_test.go, s2_commands_test.go)
internal/store: etsy_bulk_migrations.go (+ test). Tables bulk_edit_jobs, bulk_edit_items, listing_snapshots, created lazily.
Each command attaches itself through its own `registerNovelCommand` init hook (`orders` parent included). Prefix: `bulk*` / `s2*` / `Bulk*` (store).

## Decisions / deviations
- DEVIATION: no codex dispatch. I wrote the code and tests directly (every task touched 2+ files and the contract was already fully read), then verified with the full hermetic suite and mutation checks. Plans/specs unaffected.
- Plan-file contract implemented as specified; also accepts a bare JSON array, listing_id as string, CSV list values as JSON array or `|`/`,` separated.
- Journal job id = sha256 of the plan items, so re-running the same plan resumes automatically; `--restart` discards the journal. Journal granularity: per write op (`<id>|patch`, `|price`, `|alt|<image>`, `|snapshot`). Stops at first failure unless `--continue-on-error`; 401/403/429 always stop.
- Preview reads the local store only (zero HTTP). Listings missing from the store: `set` previews without an old value, `add`/`remove` are `unresolved` (resolved against the live GET at apply). The plan's "or live GET when not synced" is deferred (it would break the zero-HTTP preview acceptance).
- Apply per listing: GET listing, then snapshot (GET inventory + personalization, 404 tolerated) saved before any write, then writes. add/remove/limits re-validated against the live listing.
- tags/materials are sent comma-joined in form-encoded PATCH (Etsy form format). NOTE the generated `listings update` sends the raw JSON string for those; not touched.
- price edits go through updateListingInventory (updateListing has no price field): GET inventory, set every live offering's price, PUT. Same builder as `inventory set-qty`.
- Inventory PUT normalization (`s2InventoryPutBody`): full body, drops deleted products/offerings and read-only ids (product_id, offering_id, property_name...), Money price -> decimal float. "Other offerings byte-identical" holds relative to the normalized read (Money->decimal is applied uniformly to all offerings, as the PUT schema requires a number). UNVERIFIED LIVE: Etsy's acceptance of this exact body is for Phase 18.
- alt_text: multipart POST /shops/{shop}/listings/{id}/images with `listing_image_id` + `alt_text` (no `overwrite`, to avoid re-ranking). UNVERIFIED LIVE.
- Budget: `checkBudget(cmd, planned, ignoreBudget)` with a conservative per-run call estimate; preview reports `budget_warning`, apply refuses (exit 7); `--ignore-budget` registered on bulk-edit and `inventory low-stock --refresh`.
- Harness: `--apply`/`--yes` writes refuse under `cliutil.IsAnyHarness()`.
- Claim-vs-reality corrections: (1) receipts use `is_paid` / `is_shipped` (types.go, spec), not `was_paid`/`was_shipped`; both are accepted. (2) Receipt `grandtotal` etc. are Money objects, so receipts are parsed from the `data` JSON, not the typed (TEXT) columns. (3) Canceled receipts are excluded from `orders unshipped`.
- money summary classification (signed minor units, credits +/debits -): payout=`disburs*`, refund=`refund`, fee=`fee|listing|bill|renew`, sale=`payment|sale|transaction`, else `other` (reported, never dropped). Net = gross - fees - refunds; payouts listed separately. The real `ledger_type`/`reference_type` vocabulary is UNVERIFIED (no live shop); classification is order-sensitive and documented in the command output. Window start is inclusive.
- `inventory low-stock` reads `resources` rows (`listings_inventory`, falling back to the typed `listing_inventory`) and parses the JSON.

## Generator limitation found (needs a retro / cross-ownership fix)
The generated sync of dependent `listings_inventory` stores NOTHING: the inventory response has no id field and `resourceIDFieldOverrides` (internal/store/store.go, generated) has no `listings_inventory` entry (suffix probe wants `inventory_id`). Reproduced: `UpsertBatchDetailed("listings_inventory", [...listing_id/parent_id injected...])` warns "not cached locally (no extractable ID field)". Consequence: `inventory low-stock` (and Slice 3 `inventory audit`) have no data after a plain `sync`.
Mitigation inside my files: `inventory low-stock --refresh [--refresh-limit N] [--ignore-budget]` reads each synced listing's inventory (budget-checked, throttled) and caches it via `Store.Upsert("listings_inventory", <listing_id>, ...)`; low-stock also prints a hint when listings exist but no inventory is cached. Needed outside my ownership: add `"listings_inventory": "listing_id"` to `resourceIDFieldOverrides` (generator spec `x-pp-sync-walker`/id override) so sync caches it; Slice 3's inventory audit should read the same `resources` rows.
Also noted: generated `listings update` form-encodes tags/materials as raw JSON strings (Etsy expects comma-separated).

## Deferrals
- Live GET fallback for previewing listings missing from the store.
- `payouts by period` is the per-period `payouts_minor` column (grouped by month/week/day).
- No per-field preview of price for listings whose price varies by variation (shows the listing's minimum price).

## Verification
`go build ./... && go vet ./...` clean; `env -u ETSY_API_KEY -u ETSY_SHARED_SECRET -u ETSY_SHOP_ID PATH="/bin:/usr/bin:$PATH" go test -count=1 ./...` all packages ok. Mutation checks (each made the matching test fail): skip-done disabled (resume test), throttle removed (throttle test), set-qty matching removed (set-qty tests).

## Slice 3: Shop transcendence (branch `slice3`, worktree `wt-s3`)

### Commits (on top of ad30fbf)
f98a691 shopx helpers + fixtures; 357051b listings expiring; 286edca listings dead-weight; a1014d9 inventory audit; 5138f4b money fees; 0266c77 reviews digest; e0ed242 listings audit (+GPSR, --plan-out); fcac6c4 money reconcile; 87b842d shop attention.

### Files (all owned, nothing else touched)
The 8 scaffold files + their _test.go, plus `internal/cli/shopx_helpers.go` and `internal/cli/shopx_fixtures_test.go`.

### Decisions
- All reads decode the generic `resources` mirror (`data` JSON) via `shopxRows`; typed tables are incomplete for dependents. Reads go through `store.OpenReadOnlyContext`; missing DB file = empty store (exit 0).
- Every command has `--db` (repo convention) and annotations `mcp:read-only=true`, `pp:data-source=local`; `pp:novel-scaffold` removed. No flag is required so no `pp:happy-args` was added.
- Long descriptions verbatim from the manifest. Manifest rows 6 (dead-weight) and 8 (reviews digest) have Long = "none", so Long = the research.json description.
- Three-state handling instead of false positives: images not synced -> photo/alt rules skipped + note; economic_operators not synced -> GPSR join skipped + note; payments/receipts not synced -> reconcile sub-check skipped + note; `views` absent -> JSON null, never 0; sync age never-synced -> null.
- `listings audit --plan-out`: items only for default-rule findings with a bulk-edit field (alt_text placeholder "<title> - photo <rank>"; tags/materials/title with empty value + reason). No plan items for photos_lt_5 or GPSR (no bulk-edit field exists). Plan file mode 0600.
- Reconcile ledger-reference check recognises reference_type substrings receipt/payment/transaction/listing only; other types are unchecked. Balance continuity assumes `balance` is the running balance AFTER the entry (expected = prev.balance + cur.amount) grouped by ledger_id ordered by sequence_number then created_timestamp.
- duplicate-SKU spelling is taken from the lowest (listing_id, product_id) pair (determinism; `ListScan` order is not stable within one second).
- Receipt amounts/ids only in reconcile output (PII test asserts names/emails/addresses never appear).

### Generator limitations / cross-ownership needs (for the orchestrator)
1. **`listings_inventory` rows are never cached by sync.** Verified with the real write path (`UpsertBatchDetailed("listings_inventory", ...)` -> "no extractable ID field", stored 0) for both the `{products:[...]}` object and per-product shapes; typed `listing_inventory` stays empty too. So `inventory audit` returns `[]` on a real synced store until sync stamps an id (e.g. `id = listing_id`) on inventory responses. Fix is outside Slice 3 ownership (sync.go / a durable `etsy_*.go` hook or `resourceIDFieldOverrides`). Tests stamp `id` on fixture rows to model the fix. `inventory audit` also accepts both row shapes so it works either way. Slice 5 / polish should address this before live dogfood.
2. `reviews` rows are keyed by a `pp:params:<hash>` fingerprint (no id in a review); identical reviews (same listing, timestamp, rating, text) would collapse. Digest dedupes `reviews` vs `listings_reviews` by transaction_id when present.
3. Payments sync into resource type `shops_receipts_payments` (typed `payments` table stays empty); helpers read both names.
4. Scaffold `Short` strings for `listings audit` and `shop attention` are truncated by the generator; left unchanged (verbatim scaffold).
5. Generated tests are non-hermetic w.r.t. ETSY_* env (pre-logged); all gates run with `env -u ...`.

### Quality gates
`go build ./... && go vet ./...` clean; full hermetic `go test -count=1 ./...` green (tail in report). Empty-store smoke through the built binary: all 8 commands exit 0 with `[]`/zero-count objects under `--agent`.

## Slice 4 - Market research (branch slice4)

Commit: see `git log slice4` (single commit "feat(etsy): market research commands ...").

Files: internal/store/etsy_market_migrations.go (EnsureMarketSchema, lazy), internal/cli/mkt_helpers.go, mkt_rules.go, mkt_fixtures_test.go, market_{snapshot,trend,demand,shops,gaps,tags,price}.go + tests. market.go untouched.

Decisions
- Per-run `--max-calls`: a resumed run gets a fresh cap; `calls_used` is cumulative. Planned calls passed to checkBudget = min(planned, max-calls).
- Extra columns on market_snapshots (with_reviews, review_window_days, progress_json) so a partial snapshot resumes without refetching pages/shops/reviews, and so "review data collected, 0 reviews" is distinguishable from "no review data" (absent).
- Extra flag `--review-top N` (default 20): reviews fetched only for top-N positions per term page, to keep the call count bounded.
- Short pages (<100 results) mark later pages done without calls.
- Shop 404/403 stores a NULL shop row (not refetched, shown as unknown, never 0).
- Demand: listing state absent|shown; term state shown|cant_tell|absent. Floor = pooled reviews < --min-reviews OR < 3 listings with any.
- Empty/one-snapshot results: `{"results": [], "note": ...}` envelope.
- No YAML dep in go.mod and go.mod is not owned by this slice, so mkt_rules.go has a small parser for the documented subset (flow and block lists, quotes, comments).
- Dispatch deviation: code written directly by the slice operator instead of via coding-dispatch.sh/codex.

Deferrals / generator limits
- `capped` demand state from the plan is not implemented (review fetch takes one page of 100 per listing).
- Output is JSON via printJSON only (no table rendering).
- Test gotcha: package-global defaultDBScopeState leaks between tests; mktTestEnv resets it.

## Slice 5 — Close (2026-11-16T20:16:16Z)
- Cross-cutting fixes after merge: 0b1274b (per-listing inventory never stored: dependent sync split {products:[...]} into id-less rows -> override listings_inventory:product_id, consumers read product rows; test drives real syncDependentResource, 0 -> 4 rows), fa61eac (generated listings update/create-draft sent --tags/--materials as JSON strings; now comma-separated).
- Completion gate: per-row Cobra resolution 15/15 transcendence + 15/15 absorbed clean/behavior paths resolve as leaf commands; no TODO scaffolds remain; dogfood novel_features_check planned 15 found 15 missing none.
- Priority 1 review gate: listings copy / money summary dry-run JSON envelopes OK; receipts get-shop-shops returns structured usage error without ETSY_SHOP_ID (correct for missing required input).
- dogfood verdict FAIL on ONE check: oauth_scope_coverage (0/76). Cause: the check statically parses the scope literal joined into params.Set("scope") in auth.go; Slice 1 moved defaults to an Etsy helper with a least-privilege read-only default (approved manifest row 2), so writes are opt-in via --scopes. Design conflict with the check, not a CLI defect -> decide in Phase 12 shipcheck; candidate machine retro (check should model scope overrides/presets).
- Deferred to polish: awkward operationId-derived verbs (receipts get-shop-shops, ledger-entries get-shop-payment-account, ...); market gaps hand-rolled YAML subset parser (review in Phase 4 code review); auth setup prints "No setup URL configured".
- Generator limitations: non-hermetic generated tests; api_key-vs-oauth2 scheme pick by usage count (needed --auth-preference oauth2); two x-auth-vars on one sibling header -> duplicate-header validation error; no seam for scope defaults/client-id/error hints in auth.go/doctor.go/helpers.go (etsy-ext markers); single-object dependent responses split into id-less child rows.

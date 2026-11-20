---
name: pp-etsy
description: "Etsy Open API v3 commands plus a local SQLite store for offline fee reports, ledger reconciliation, listing and GPSR audits, market research, and previewed bulk edits. Trigger phrases: `what are my etsy fees`, `which etsy listings are expiring`, `audit my etsy listings`, `is my etsy shop gpsr ready`, `research my etsy niche`, `what do competitors charge on etsy`, `reconcile my etsy payouts`, `use etsy`, `run etsy-pp-cli`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - etsy-pp-cli
    install:
      - kind: go
        bins: [etsy-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/etsy/cmd/etsy-pp-cli
---

# Etsy — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `etsy-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install etsy --cli-only
   ```
2. Verify: `etsy-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.11 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/etsy/cmd/etsy-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Log in, set ETSY_SHOP_ID, run `etsy-pp-cli sync`, then answer fulfilment, money and catalog questions offline with `money fees`, `money reconcile`, `listings audit --rules gpsr`, `listings expiring`, `inventory audit` and `orders unshipped`. Public market research (`market snapshot`, then `market shops`, `gaps`, `tags`, `price`) needs only the API key. The default login is read-only; writes need `auth login --all-scopes` and an explicit `--yes`, and `listings bulk-edit` previews every change until `--apply --yes`.

## When to Use This CLI

Use this CLI for an Etsy seller's shop operations: reports and audits over the whole catalog, fee and payout questions, fulfilment queues, and carefully previewed bulk edits. It is the right choice when an agent needs answers across many listings or orders without spending API quota or risking rate-limit bans. It also does budgeted market research from public data: snapshots of search results, demand proxies, competitor shops, gaps and price bands, with trends between snapshots.

## Anti-triggers

Do not use this CLI for:
- Do not use it to buy on Etsy or manage a buyer cart or favorites; the v3 API has no buyer cart endpoints.
- Do not use it for Etsy Ads, Marketplace Insights or Offsite Ads stats; those are not in Open API v3.
- Do not use it for high-frequency monitoring or polling; Etsy bans apps for it.

## Unique Capabilities

### Catalog hygiene
- **`listings audit`** — Find every listing with missing tags, materials, alt text or photos, or missing GPSR manufacturer/responsible-person data, in one offline pass.

  _Reach for this before bulk edits or the 2027-02-01 GPSR deadline instead of paging every listing live. Needs `etsy-pp-cli auth login`, ETSY_SHOP_ID and a prior `etsy-pp-cli sync`. `--plan-out plan.json` writes a plan that `listings bulk-edit --plan plan.json` previews._

  ```bash
  etsy-pp-cli listings audit --rules gpsr --agent
  ```
- **`listings expiring`** — List active listings expiring soon and whether they auto-renew.

  _Use to plan renewal fees and avoid listings silently going inactive. Needs `etsy-pp-cli auth login`, ETSY_SHOP_ID and a prior `etsy-pp-cli sync`._

  ```bash
  etsy-pp-cli listings expiring --within 30d --agent
  ```
- **`inventory audit`** — Spot zero-quantity variations, missing or duplicate SKUs and price spread across variations.

  _Use for variation and SKU data quality; use inventory low-stock for a reorder list. Needs `etsy-pp-cli auth login`, ETSY_SHOP_ID and a prior `etsy-pp-cli sync`._

  ```bash
  etsy-pp-cli inventory audit --agent
  ```
- **`listings dead-weight`** — Find active listings with no sales in a window, with favorite counts.

  _Use to decide what not to renew or to rework. Needs `etsy-pp-cli auth login`, ETSY_SHOP_ID and a prior `etsy-pp-cli sync`. Returns [] with a note when transactions were never synced (unknown is not zero)._

  ```bash
  etsy-pp-cli listings dead-weight --since 90d --agent
  ```

### Money and fees
- **`money fees`** — See what Etsy charged you, by fee type, for any period.

  _Use for fee and cost questions without spending API quota. Needs `etsy-pp-cli auth login`, ETSY_SHOP_ID and a prior `etsy-pp-cli sync`. (ledger needs the transactions_r scope)_

  ```bash
  etsy-pp-cli money fees --since 30d --agent
  ```
- **`money reconcile`** — Match receipts, payments and ledger entries by ID and compare receipt totals with summed payment amounts and currencies.

  _Use when payouts or refunds look wrong, before opening Etsy support. Needs `etsy-pp-cli auth login`, ETSY_SHOP_ID and a prior `etsy-pp-cli sync`. Notes name any family that was never synced._

  ```bash
  etsy-pp-cli money reconcile --since 30d --agent
  ```

### Agent-native plumbing
- **`shop attention`** — One compact summary of what needs attention: unshipped orders, expiring listings, audit findings, reconcile mismatches and sync age.

  _Agents should call this first for a daily check instead of polling the API. Needs `etsy-pp-cli auth login`, ETSY_SHOP_ID and a prior `etsy-pp-cli sync`. Read `notes`: a 0 for a family that was never synced means unknown, not zero._

  ```bash
  etsy-pp-cli shop attention --agent
  ```
- **`reviews digest`** — Rating histogram, low-rated reviews and per-listing averages for a period.

  _Use to catch product problems surfacing in reviews. Needs ETSY_SHOP_ID and a prior `etsy-pp-cli sync` (the default sync includes reviews; works with the API key alone; no OAuth)._

  ```bash
  etsy-pp-cli reviews digest --since 30d --agent
  ```

### Sync and resumable edits

`sync` mirrors shop data with call budgets, incremental receipt payment checks and saved progress. Capped runs exit 7; scope failures may exit 1. See [Budgets, resuming and privacy](#budgets-resuming-and-privacy) for the `/payments` 403 verification rule, `--strict`, and rolling 24h quota recovery. `doctor` includes a quota row for the selected API key.

```bash
etsy-pp-cli sync --max-calls 100 --agent
etsy-pp-cli doctor --json
etsy-pp-cli listings bulk-edit --listings 1000000001 --field tags --add "linen apron"
```

Bulk edit previews before applying. Continue a job with `--resume JOB`; deprecated `--restart` and `--resume` together exit 2. See the audit-and-preview recipe before applying a plan.

### Analytics and command discovery

`analytics` reads synced data without writing or migrating the store and refuses newer schemas. A missing store stays empty without creating files; upgrade an older schema with sync first. `--type` accepts stored spellings, including kebab-case partitions, and `resource_type` echoes the stored name. `--group-by` preserves exact scalar values, including case and whitespace; null or missing values are JSON null (shown as `(none)` in tables, while a literal `(none)` is quoted). Unknown types or grouping fields exit 2. Output defaults to JSON when piped or under MCP.

```bash
etsy-pp-cli analytics --type listings --group-by state --limit 10 --json
etsy-pp-cli which "compare two snapshots" --json
```

`which` finds commands by capability; snapshot comparison and trend questions route to `market trend`. No confident match exits 2.

### Market research
Snapshot selection: explicit `--snapshot`, `--from` and `--to` must be positive IDs (otherwise exit 2). Trend accepts both `--from` and `--to` or neither; missing explicit snapshots exit 3, including with no store. Omitted IDs select the latest complete snapshot (trend uses the latest two). Snapshot `is_digital` derives from `listing_type`: download/both → true, physical → false, unknown → null; existing snapshots keep null.

```bash
etsy-pp-cli market trend --from 1 --to 2 --agent
etsy-pp-cli market tags --snapshot 1 --json
etsy-pp-cli market demand --snapshot 1 --compact --agent
etsy-pp-cli market tags --snapshot 1 --agent --select results.tags
```

Explicit `--compact` projects fixed report row keys; `--agent`'s implied compact does not trigger that projection. `--select` takes precedence and advisory notes remain. Nested tags/trend reports refuse both `--csv` and `--plain` with exit 2.

Market commands emit `{results, notes}`; `market tags` payload is `results.tags` / `results.title_words`; `market trend` payload is under `results` (e.g. `results.new_listings`). --select paths are relative to the envelope even under --agent (e.g. --agent --select results.tags); the output then appears at results.results.tags. Don't mix anchored (results.x) and unanchored (x) paths in one --select. demand listings_total is null for not_fetched terms. Demand states include no_results (fetched with no matches) and not_fetched (resume needed). Shop shop_name is nullable when unknown. --csv and --plain are refused for the nested market reports (tags, trend); use --json or --agent.

- **`market snapshot`** — Capture a dated snapshot of the listings and shops that appear for your search terms.

  _Start any market or competitor question here; the other market commands read its stored snapshots. Uses only the API key. Each search page can cost up to 1 + --per-page calls (101 at the default) against the 5,000/day quota; use --per-page 25 for a cheaper scan, and --with-reviews when you need demand._

  ```bash
  etsy-pp-cli market snapshot --terms terms.txt --pages 2 --max-calls 200
  ```

- **`market trend`** — See which listings and shops entered or left, and how prices, favorites and positions moved, between two snapshots.

  _Use to compare two snapshots: what changed between snapshots, which listings entered or left, which shops entered or left, and price, favorites and position moves over time. Needs two complete snapshots; with fewer it returns `results: []` plus a note._

  ```bash
  etsy-pp-cli market trend --agent
  ```
- **`market demand`** — Rank listings and search terms by recent review volume and shop sales, with an explicit can't-tell result for thin data.

  _Use to estimate which products sell best without third-party tools. Needs a snapshot taken with `--with-reviews`; otherwise every listing reports `absent` (no review data, not zero)._

  ```bash
  etsy-pp-cli market demand --window 12mo --agent
  ```
- **`market shops`** — Profile the shops behind a snapshot and see whether big shops crowd out new ones.

  _Use to size up competitors in a niche. Reads the latest complete market snapshot._

  ```bash
  etsy-pp-cli market shops --agent
  ```
- **`market gaps`** — Find targets where no matching listing was found in the pages and terms you searched, with coverage checked against known competitors.

  _Use to find product ideas with little competition, stated as what was searched. Needs a prior market snapshot and a --rules YAML or JSON file of targets. Over MCP, pass rules inline with --rules-json and control ids with --control-ids (JSON and ids each up to 96 KiB; on Windows the whole command must fit ~31 KiB); each page re-sends the inline content (stateless paging)._

  ```bash
  etsy-pp-cli market gaps --rules classes.yaml --agent
  ```
- **`market tags`** — See which tags and title words the best-selling listings use that the rest don't.

  _Use when writing titles and tags for a new listing. Payload is under `results.tags` and `results.title_words`. Reads the latest snapshot; ranks by review demand when collected, otherwise by search position (reported in `results.basis`)._

  ```bash
  etsy-pp-cli market tags --agent
  ```

Market rules (gaps and price classes): `all: ["linen", "apron"]` requires BOTH regexes (AND), alongside any include and no exclude. `fields` defaults to `[title, tags]`; `[title]` (or YAML scalar `title`) restricts matching to title. Empty, duplicate, unknown fields and `[tags]` alone exit 2. `fold: true` lowercases text and collapses runs outside Unicode letters, digits and `%` to one space; patterns stay unchanged, and removed literal punctuation is rejected with the removed characters identified. Default fold is false. Targets with neither include nor all patterns match every listing not excluded, with `matches_all: true` and an advisory in `notes[]`. JSON rejects unknown keys at the top level and per target. The 256-pattern total includes include, exclude and all; each pattern is limited to 512 bytes. Fold treats combining marks as separators; pre-composed (NFC) text is expected.

- **`market price`** — Price ranges per category and whether price tracks demand.

  _Use when pricing a new product against the market. Reads the latest snapshot; the price-vs-demand correlation is null unless the snapshot used --with-reviews. Over MCP, --group-by class takes its rules inline with --rules-json (JSON, ≤96 KiB; on Windows the whole command must fit ~31 KiB); each page re-sends the inline content (stateless paging)._

  ```bash
  etsy-pp-cli market price --agent
  ```

### Worked market rules: AND and folding

Save this complete YAML as `classes.yaml`. `include` requires any listed regex; `all` requires every regex; `exclude` vetoes a match. This target finds linen aprons in titles, excludes ceramic mugs, and folds punctuation and case in listing text.

```yaml
targets:
  - name: linen apron
    include: [apron]
    all: [linen, apron]
    exclude: [ceramic mug]
    fields: [title]
    fold: true
```

The equivalent complete JSON is:

```json
{"targets":[{"name":"linen apron","include":["apron"],"all":["linen","apron"],"exclude":["ceramic mug"],"fields":["title"],"fold":true}]}
```

Keys are exact and case-sensitive: the root accepts only `targets`; each target accepts `name`, `include`, `exclude`, `all`, `fields`, `fold`. Unknown JSON keys exit 2. With folding, write `linen apron`, not `linen-apron`: removed punctuation literals, including literals inside classes or alternatives, and repeated literal spaces are rejected rather than rewritten. Empty, duplicate or unsupported fields, including tags-only matching, are rejected. Fold normalizes text, not regex source.

```bash
etsy-pp-cli market gaps --rules classes.yaml --agent
etsy-pp-cli market price --group-by class --rules classes.yaml --agent
etsy-pp-cli market gaps --rules-json '{"targets":[{"name":"linen apron","all":["linen","apron"],"fields":["title"],"fold":true}]}' --control-ids 1000000001 --agent
etsy-pp-cli market price --group-by class --rules-json '{"targets":[{"name":"linen apron","all":["linen","apron"],"fold":true}]}' --agent
```

### Local data and privacy

Local review writes strip `buyer_user_id` and `image_url_fullxfull` from `reviews` and `listings_reviews`. The next writable store open scrubs those fields from legacy JSON and typed review columns; read-only opens do not migrate. Review text and transaction links remain buyer-bearing and restricted over MCP. This does not erase backups or old WAL frames.

`local-data prune --older-than 2160h [--receipts] [--reviews]` previews counts only; add `--yes` to apply. With neither scope flag it selects both. Global `--dry-run` and harness runs stay previews. Shipped receipts are pruned as a unit with transactions, payments and payment checks; unshipped receipts, receipts with unknown shipped state, and rows without timestamps are kept. Applied pruning persists a retention floor. Never-stored old shipped receipts are rejected; stored updates proceed until the next prune. Receipt dependents are rejected for remembered receipt IDs, including floor-rejected receipts and pruned orphan parents, regardless of their own age. Receipt pruning leaves every review untouched and reports `reviews_unlinkable_after_prune` when removing its only transaction link prevents later buyer erasure; add `--reviews` to prune old reviews too. Only --reviews sets the reviews floor: it prunes reviews by their own age and rejects never-stored older reviews; stored review updates pass. Write-time policy drops never delete stored rows. Orphan purchase rows use their own age at prune, with an explicit null or zero shipped_timestamp keeping them. `local-data prune --clear-floor [--receipts] [--reviews] --yes` removes the selected floor. With `--receipts`, it also clears remembered receipt IDs and reports their count: clearing re-admits previously pruned receipts on the next sync when no suppression applies. A re-sync restores a removed row only when no active retention floor or suppression excludes it.

`local-data erase-buyer --buyer-user-id ID | --buyer-email EMAIL | --receipt-id ID` accepts repeatable selectors and previews counts unless `--yes` is passed. `--receipt-id` erases and suppresses the receipt’s whole buyer when a positive buyer ID is known; otherwise the receipt ID is durably suppressed. Use `--unsuppress --receipt-id ID` to remove receipt suppression. Prefer `--buyer-email -` to read the email from stdin and keep it out of shell history. Emails match after trimming and lowercasing, resolve to buyer IDs, and erase those buyers' receipts and dependents, including reviews linked by transaction ID. Output never includes emails or matched records. Seller `user_addresses` are excluded. IDs must be positive; known token-owner IDs are refused. The owner is identified from the configured OAuth token's numeric prefix, an explicitly identified current-user row, receipt seller IDs, or the configured shop’s cached owner ID. If none is available, output reports that own-id refusal is unavailable; verify the target yourself.

Erasure keeps `etsy_erasure_suppression(buyer_user_id, erased_at)`: minimal personal data retained to honour erasure. The shared write hook rejects incoming buyer data with that buyer ID and a creation timestamp at or before erasure; newer data is allowed. `local-data erase-buyer --unsuppress ID --yes` removes suppression. Suppressed records without a parseable creation timestamp are rejected. Missing buyer identity can prevent suppression; data already scrubbed of buyer fields requires its transaction link to remain locally available for selection. Guest erasure retains receipt IDs and transaction-to-receipt links to block re-entry. Positive-buyer erasure retains a receipt-to-buyer hint for unsuppression diagnostics. `--unsuppress --receipt-id ID` clears guest receipt and transaction suppression; for a receipt belonging to a suppressed buyer it makes no change and directs you to `--unsuppress <buyer>`. Retention floors and suppression are local policies, not requests to Etsy.

Both commands always refuse MCP, including dry runs and operator buyer-data opt-ins. They process all matching `data*.db` stores in the resolved data directory (plus an explicit `--db`), report each basename, clean matching resource learnings and buyer learn families, clear recall misses, rewrite matching teach.log entries, and remove the HTTP cache on apply. The non-personal `payments_ever_succeeded` flag survives. Stop concurrent sync/teach writers first; separate stores and filesystem cleanup cannot commit atomically together.

Erasure vacuums by default; `--no-vacuum` skips reclamation and `--vacuum` remains a compatibility alias. Prune requires `--vacuum`. Applies use secure deletion on the pinned transaction connection. The reclamation sequence optimizes FTS, rebuilds external-content FTS before VACUUM, then truncates the WAL. A busy VACUUM reports `vacuum: "skipped_busy"`; a busy checkpoint reports `vacuum: "checkpoint_busy"` (exit 0); each incomplete status prints “purged; vacuum incomplete (…) — rerun with --vacuum when the store is idle”. Cross-store status reports the worst result: error, skipped_busy, checkpoint_busy, optimize_busy, completed. Busy FTS steps report skipped_busy without failing committed erasure. Close readers and rerun with `--vacuum`. Write-time policy rejection leaves stored rows untouched. Only the implicit erase vacuum skips stores with zero victims; an explicit `--vacuum` still runs. Without a completed vacuum, free pages or old WAL frames may retain bytes. Exports, custom stores not selected by `--db`, backups and Time Machine are out of reach; remove those copies separately. Filesystem snapshots, SSD remapping and concurrent writers prevent a forensic-erasure guarantee.

Both selective commands describe `--db` as “Additional store file to process (DataDir stores are always processed)”. It adds a file rather than replacing the default selection. Email stdin accepts one address per line. `kept_unlinkable_reviews` reports stored reviews with neither buyer identity nor a locally resolvable transaction; these may survive erasure.

Taught `entity_lookups` and learn-journal `unresolved_entities` originate in operator input and are not erased. Platform profile tenant stores live under `clients/<profile>/<cli>/data.db` (see `internal/platform/profile.go`) and are outside the flat DataDir scan unless explicitly added with `--db`. Their `responses` caches are outside the normal `http` cache cleanup and must be removed separately.

After unsuppression, re-sync can restore receipts, transactions and reviews subject to retention floors; `--unsuppress BUYER` cannot be combined with `--receipt-id`.

Preview selective retention, erasure or policy removal before applying:

```bash
etsy-pp-cli local-data prune --older-than 2160h --receipts
etsy-pp-cli local-data prune --older-than 2160h --receipts --yes --vacuum
etsy-pp-cli local-data prune --older-than 2160h --reviews --json
etsy-pp-cli local-data prune --clear-floor --receipts
etsy-pp-cli local-data prune --clear-floor --receipts --yes
etsy-pp-cli local-data erase-buyer --buyer-user-id 1000000002
etsy-pp-cli local-data erase-buyer --receipt-id 1000000001
etsy-pp-cli local-data erase-buyer --unsuppress 1000000002
etsy-pp-cli local-data erase-buyer --unsuppress 1000000002 --yes
```

### MCP limits

`PP_MCP_MAX_BYTES` controls the output budget for all tools. See [Paging MCP results](#paging-mcp-results) for cursor fields, previews and limits. `PP_MCP_MAX_CONCURRENCY` defaults to 4 mirrored children (range 1–32); read-only mirrors annotated `pp:data-source=local` use a separate cap of four times that value. Native tools bypass mirror admission. `PP_MCP_QUEUE_TIMEOUT` defaults to `20s` (range `1ms`–`5m`); timeout returns a structured busy error without partial output. Invalid settings warn and fall back to defaults. Settings are read at registration. Stdio has five workers, so raising the mirror cap alone cannot raise stdio parallelism beyond five.

Buyer-bearing local reads refuse or filter data by default; see [Paging MCP results](#paging-mcp-results) for the operator opt-in and buyer paging limits. Retention and erasure always refuse MCP, even with the opt-in.

```bash
PP_MCP_MAX_BYTES=60000 PP_MCP_MAX_CONCURRENCY=4 PP_MCP_QUEUE_TIMEOUT=20s etsy-pp-mcp
```

## Command Reference

- `etsy-pp-cli analytics --type listings --group-by state --json` — Read-only stored analytics.
- `etsy-pp-cli local-data prune --older-than 2160h --receipts` — Preview retention.
- `etsy-pp-cli local-data erase-buyer --buyer-user-id 1000000002` — Preview erasure.

**app** — Manage app

- `etsy-pp-cli app ping` — Check to confirm connectivity to the Etsy API with an application
- `etsy-pp-cli app token-scopes` — Check the scopes of the provided token

**buyer_taxonomy** — Manage buyer taxonomy

- `etsy-pp-cli buyer-taxonomy get-nodes` — Retrieves the full hierarchy tree of buyer taxonomy nodes.
- `etsy-pp-cli buyer-taxonomy get-properties-by-id` — Retrieves a list of product properties, with applicable scales and values, supported for a specific buyer taxonomy ID.

**economic_operators** — Manage economic operators

- `etsy-pp-cli economic-operators create-shop` — Creates a new economic operator (a manufacturer or EU responsible person record) for a specific shop
- `etsy-pp-cli economic-operators get-shop` — Retrieves all economic operators (manufacturer and EU responsible person records) of a specific shop
- `etsy-pp-cli economic-operators get-shop-shops` — Retrieves a single economic operator, referenced by economic operator ID and shop ID.
- `etsy-pp-cli economic-operators update-shop` — Updates an existing economic operator for a specific shop.

**holiday_preferences** — Manage holiday preferences

- `etsy-pp-cli holiday-preferences get` — Retrieves a list of holidays that are available to a shop to set a preference for.
- `etsy-pp-cli holiday-preferences update` — Updates the preference for whether the seller will process orders or not on the holiday.

**ledger_entries** — Manage ledger entries

- `etsy-pp-cli ledger-entries get-shop-payment-account` — Get a Shop Payment Account Ledger's Entries
- `etsy-pp-cli ledger-entries get-shop-payment-account-ledger-entry` — Get a single Shop Payment Account Ledger's Entry

**listing_files** — Manage listing files

- `etsy-pp-cli listing-files delete` — Deletes a file from a specific listing.
- `etsy-pp-cli listing-files get` — Retrieves a single file associated with the given digital listing.
- `etsy-pp-cli listing-files get-all` — Retrieves all the files associated with the given digital listing.
- `etsy-pp-cli listing-files upload` — Uploads a new file for a digital listing, or associates an existing file with a specific listing.

**listing_images** — Manage listing images

- `etsy-pp-cli listing-images delete` — Open API V3 endpoint to delete a listing image.
- `etsy-pp-cli listing-images get` — Retrieves all listing image resources for a listing with a specific listing ID.
- `etsy-pp-cli listing-images get-listings` — Retrieves the references and metadata for a listing image with a specific image ID.
- `etsy-pp-cli listing-images upload` — Uploads or assigns an image to a listing identified by a shop ID with a listing ID.

**listing_inventory** — Manage listing inventory

- `etsy-pp-cli listing-inventory get` — Retrieves the inventory record for a listing. Listings you did not edit using the Etsy.
- `etsy-pp-cli listing-inventory get-listing-product` — Open API V3 endpoint to retrieve a ListingProduct by ID.
- `etsy-pp-cli listing-inventory get-listings-inventory-by-listing-ids` — Retrieves the inventory record for each listing referenced by listing ID. Requires the `listings_r` OAuth scope.
- `etsy-pp-cli listing-inventory update` — Updates the inventory for a listing identified by a listing ID.

**listing_personalization** — Manage listing personalization

- `etsy-pp-cli listing-personalization delete` — Deletes personalization for a listing.
- `etsy-pp-cli listing-personalization get` — Retrieves a listing's personalization questions by listing ID.
- `etsy-pp-cli listing-personalization update` — Creates or updates personalization settings for a listing

**listing_translations** — Manage listing translations

- `etsy-pp-cli listing-translations create` — Creates a ListingTranslation by listing_id and language
- `etsy-pp-cli listing-translations get` — Get a Translation for a Listing in the given language
- `etsy-pp-cli listing-translations update` — Updates a ListingTranslation by listing_id and language

**listing_variation_images** — Manage listing variation images

- `etsy-pp-cli listing-variation-images get` — Gets all variation images on a listing.
- `etsy-pp-cli listing-variation-images update-variation-images` — Creates variation images on a listing.

**listing_videos** — Manage listing videos

- `etsy-pp-cli listing-videos delete` — Open API V3 endpoint to delete a listing video.
- `etsy-pp-cli listing-videos get` — Retrieves all listing video resources for a listing with a specific listing ID.
- `etsy-pp-cli listing-videos get-listings` — Retrieves a single video associated with the given listing. Requesting a video from a listing returns an empty result.
- `etsy-pp-cli listing-videos upload` — Uploads a new video for a listing, or associates an existing video with a specific listing.

**listings** — Manage listings

- `etsy-pp-cli listings create-draft` — Creates a physical draft [listing](/documentation/reference#tag/ShopListing) product in a shop on the Etsy channel.
- `etsy-pp-cli listings delete` — Open API V3 endpoint to delete a ShopListing.
- `etsy-pp-cli listings delete-property` — Deletes a property for a Listing.
- `etsy-pp-cli listings find-all-active` — A list of all active listings on Etsy paginated by their creation date.
- `etsy-pp-cli listings find-all-active-by-shop` — Retrieves a list of all active listings on Etsy in a specific shop, paginated by listing creation date.
- `etsy-pp-cli listings get` — Retrieves a listing record by listing ID.
- `etsy-pp-cli listings get-by-ids` — Allows to query multiple listing ids at once. Limit 100 ids maximum per query.
- `etsy-pp-cli listings get-by-shop` — Endpoint to list Listings that belong to a Shop. Listings can be filtered using the 'state' param.
- `etsy-pp-cli listings get-by-shop-receipt` — Gets all listings associated with a receipt.
- `etsy-pp-cli listings get-by-shop-return-policy` — Gets all listings associated with a Return Policy.
- `etsy-pp-cli listings get-by-shop-section-id` — Retrieves all the listings from the section of a specific shop.
- `etsy-pp-cli listings get-featured-by-shop` — Retrieves Listings associated to a Shop that are featured.
- `etsy-pp-cli listings get-properties` — Get a listing's properties
- `etsy-pp-cli listings get-property` — Development for this endpoint is in progress. It will only return a 501 response. Retrieves a listing's property
- `etsy-pp-cli listings get-shipping-by-ids` — Retrieves the shipping profile for each listing referenced by listing ID. Requires the `shops_r` OAuth scope.
- `etsy-pp-cli listings update` — Updates a listing, identified by a listing ID, for a specific shop identified by a shop ID.
- `etsy-pp-cli listings update-property` — Updates or populates the properties list defining product offerings for a listing.

**payments** — Manage payments

- `etsy-pp-cli payments get` — Retrieves a list of payments from a shop identified by `shop_id`.
- `etsy-pp-cli payments get-account-ledger-entry` — Get a Payment from a PaymentAccount Ledger Entry ID, if applicable
- `etsy-pp-cli payments get-shop-by-receipt-id` — Retrieves a payment from a specific receipt, identified by `receipt_id`, from a specific shop, identified by `shop_id`

**processing_profiles** — Manage processing profiles

- `etsy-pp-cli processing-profiles create-shop-readiness-state-definition` — Creates a new ReadinessStateDefinition.
- `etsy-pp-cli processing-profiles delete-shop-readiness-state-definition` — Deletes a ReadinessStateDefinition by given readiness state definition ID.
- `etsy-pp-cli processing-profiles get-shop-readiness-state-definition` — Retrieves a ProcessingProfile referenced by readiness state definition ID.
- `etsy-pp-cli processing-profiles get-shop-readiness-state-definitions` — Retrieves a list of ProcessingProfiles available in the specific Etsy shop identified by its shop ID.
- `etsy-pp-cli processing-profiles update-shop-readiness-state-definition` — Updates an existing ReadinessStateDefinition.

**production_partners** — Manage production partners

- `etsy-pp-cli production-partners <shop_id>` — Retrieves a list of production partners available in the specific Etsy shop identified by its shop ID.

**receipts** — Manage receipts

- `etsy-pp-cli receipts create-shipment` — Submits tracking information for a Shop Receipt, which creates a Shop Receipt Shipment entry for the given receipt_id.
- `etsy-pp-cli receipts get-shop` — Requests the Shop Receipts from a specific Shop, unfiltered or filtered by receipt id range or offset, date, paid
- `etsy-pp-cli receipts get-shop-shops` — Retrieves a receipt, identified by a receipt id, from an Etsy shop.
- `etsy-pp-cli receipts update-shop` — Updates the status of a receipt, identified by a receipt id, from an Etsy shop.

**return_policies** — Manage return policies

- `etsy-pp-cli return-policies consolidate-shop` — Consolidates Return Policies by moving all listings from a source return policy to a destination return policy
- `etsy-pp-cli return-policies create-shop-return-policy` — Creates a new Return Policy.
- `etsy-pp-cli return-policies delete-shop-return-policy` — Deletes an existing Return Policy.
- `etsy-pp-cli return-policies get-shop` — Returns a shop's list of existing Return Policies
- `etsy-pp-cli return-policies get-shop-return-policy` — Retrieves an existing Return Policy.
- `etsy-pp-cli return-policies update-shop-return-policy` — Updates an existing Return Policy.

**reviews** — Manage reviews

- `etsy-pp-cli reviews get-by-listing` — Open API V3 to retrieve the reviews for a listing given its ID.
- `etsy-pp-cli reviews get-by-shop` — Open API V3 to retrieve the reviews from a shop given its ID.

**seller_taxonomy** — Manage seller taxonomy

- `etsy-pp-cli seller-taxonomy get-nodes` — Retrieves the full hierarchy tree of seller taxonomy nodes.
- `etsy-pp-cli seller-taxonomy get-properties-by-taxonomy-id` — Retrieves a list of product properties, with applicable scales and values, supported for a specific seller taxonomy ID.

**shipping_profiles** — Manage shipping profiles

- `etsy-pp-cli shipping-profiles create-shop` — Creates a new ShippingProfile.
- `etsy-pp-cli shipping-profiles create-shop-destination` — Creates a new shipping destination, which sets the shipping cost, carrier
- `etsy-pp-cli shipping-profiles create-shop-upgrade` — Creates a new shipping profile upgrade, which can establish a price for a shipping option
- `etsy-pp-cli shipping-profiles delete-shop` — Deletes a ShippingProfile by given id.
- `etsy-pp-cli shipping-profiles delete-shop-destination` — Deletes a shipping destination and removes the destination option from every listing that uses the associated shipping
- `etsy-pp-cli shipping-profiles delete-shop-upgrade` — Deletes a shipping profile upgrade and removes the upgrade option from every listing that uses the associated shipping
- `etsy-pp-cli shipping-profiles get-shipping-carriers` — Retrieves a list of available shipping carriers and the mail classes associated with them for a given country
- `etsy-pp-cli shipping-profiles get-shop` — Retrieves a list of shipping profiles available in the specific Etsy shop identified by its shop ID.
- `etsy-pp-cli shipping-profiles get-shop-destinations-by` — Retrieves a list of shipping destination objects associated with a shipping profile.
- `etsy-pp-cli shipping-profiles get-shop-shops` — Retrieves a Shipping Profile referenced by shipping profile ID.
- `etsy-pp-cli shipping-profiles get-shop-upgrades` — Retrieves the list of shipping profile upgrades assigned to a specific shipping profile.
- `etsy-pp-cli shipping-profiles update-shop` — Changes the settings in a shipping profile.
- `etsy-pp-cli shipping-profiles update-shop-destination` — Updates an existing shipping destination, which can set or reassign the shipping cost, carrier
- `etsy-pp-cli shipping-profiles update-shop-upgrade` — Updates a shipping profile upgrade and updates any listings that use the shipping profile.

**shop_sections** — Manage shop sections

- `etsy-pp-cli shop-sections create` — Creates a new section in a specific shop.
- `etsy-pp-cli shop-sections delete` — Deletes a section in a specific shop given a valid shop_section_id.
- `etsy-pp-cli shop-sections get` — Retrieves the list of shop sections in a specific shop identified by shop ID.
- `etsy-pp-cli shop-sections get-shops` — Retrieves a shop section, referenced by section ID and shop ID.
- `etsy-pp-cli shop-sections update` — Updates a section in a specific shop given a valid shop_section_id.

**shops** — Manage shops

- `etsy-pp-cli shops find` — Open API V3 endpoint for searching shops by name.
- `etsy-pp-cli shops get` — Retrieves the shop identified by a specific shop ID.
- `etsy-pp-cli shops get-by-owner-user-id` — Retrieves the shop identified by the shop owner's user ID.
- `etsy-pp-cli shops update` — Updates a shop. Assumes that all string parameters are provided in the shop's primary language.

**transactions** — Manage transactions

- `etsy-pp-cli transactions get-shop-receipt` — Retrieves a transaction by transaction ID.
- `etsy-pp-cli transactions get-shop-receipt-by-listing` — Retrieves the list of transactions associated with a listing.
- `etsy-pp-cli transactions get-shop-receipt-by-receipt` — Retrieves the list of transactions associated with a specific receipt.
- `etsy-pp-cli transactions get-shop-receipt-by-shop` — Retrieves the list of transactions associated with a shop.

**user_addresses** — Manage user addresses

- `etsy-pp-cli user-addresses get` — Open API V3 endpoint to retrieve UserAddresses for a User.
- `etsy-pp-cli user-addresses get-user-address` — Open API V3 endpoint to retrieve a UserAddress for a User.

**users** — Manage users

- `etsy-pp-cli users get` — Retrieves a user profile based on a unique user ID.
- `etsy-pp-cli users get-me` — Returns basic info for the user making the request.


**inventory**


- `etsy-pp-cli inventory audit` — Spot zero-quantity variations, missing or duplicate SKUs and price spread across variations.


**listings**


- `etsy-pp-cli listings audit` — Find every listing with missing tags, materials, alt text or photos, or missing GPSR manufacturer/responsible-person data, in one offline pass.

- `etsy-pp-cli listings expiring` — List active listings expiring soon and whether they auto-renew.

- `etsy-pp-cli listings dead-weight` — Find active listings with no sales in a window, with favorite counts.


**market**


- `etsy-pp-cli market snapshot` — Capture a dated snapshot of the listings and shops that appear for your search terms.

- `etsy-pp-cli market trend` — See which listings and shops entered or left, and how prices, favorites and positions moved, between two snapshots.

- `etsy-pp-cli market demand` — Rank listings and search terms by recent review volume and shop sales, with an explicit can't-tell result for thin data.

- `etsy-pp-cli market shops` — Profile the shops behind a snapshot and see whether big shops crowd out new ones.

- `etsy-pp-cli market gaps` — Find targets where no matching listing was found in the pages and terms you searched, with coverage checked against known competitors.

- `etsy-pp-cli market tags` — See which tags and title words the best-selling listings use that the rest don't.

- `etsy-pp-cli market price` — Price ranges per category and whether price tracks demand.


**money**


- `etsy-pp-cli money fees` — See exactly what Etsy charged you by fee type for any period.

- `etsy-pp-cli money reconcile` — Match receipts, payments and ledger entries by ID and compare receipt totals with summed payment amounts and currencies.


**reviews**


- `etsy-pp-cli reviews digest` — Rating histogram, low-rated reviews and per-listing averages for a period.


**shop**


- `etsy-pp-cli shop attention` — One compact summary of what needs attention: unshipped orders, expiring listings, audit findings, reconcile mismatches and sync age.




**shop tools (hand-written)**

- `etsy-pp-cli listings bulk-edit` — Edit many listings from a plan or flags; previews with no API calls until `--apply --yes` (needs `auth login --all-scopes`)
- `etsy-pp-cli listings copy` — Build a draft from an existing listing; preview with `--dry-run`, create with `--yes`
- `etsy-pp-cli listings snapshot` — Save a listing, inventory and personalization to the local store, or `--list` history
- `etsy-pp-cli inventory set-qty` — Change one offering's quantity; writes only with `--yes`
- `etsy-pp-cli inventory low-stock` — List offerings at or below `--threshold` from the local store
- `etsy-pp-cli orders unshipped` — Paid, unshipped receipts from the local store; PII-minimized unless `--full`
- `etsy-pp-cli money summary` — Gross, fees, refunds, payouts and net from the synced ledger

**write gate** — Generated endpoint commands that send PUT, PATCH, POST or DELETE (for example `listings delete`, `listings update`, `listing-inventory update`) refuse without `--yes` and exit 2. Run them with `--dry-run` first. `ETSY_SHOP_ID` must be set for shop data.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
etsy-pp-cli which "<capability in your own words>"
```

Trend and snapshot-comparison questions route to `market trend`.

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Market rules: AND + fold for gaps and price classes

Use the latest complete market snapshot with this synthetic target:

```bash
cat > rules.yaml <<'YAML'
targets:
  - name: linen apron
    all: [linen, apron]
    exclude: [ceramic mug]
    fields: [title]
    fold: true
YAML
etsy-pp-cli market gaps --rules rules.yaml --control-ids 1000000001 --agent
etsy-pp-cli market price --group-by class --rules rules.yaml --agent
```

`all` requires every listed regex to match, while `exclude` vetoes a match.
`fields: [title]` restricts matching to titles instead of titles plus tags.
`fold` lowercases listing text and collapses runs outside Unicode letters, digits and % to one space; regex patterns stay unchanged.

Pass the same rule inline for MCP:

```bash
etsy-pp-cli market gaps --rules-json '{"targets":[{"name":"linen apron","all":["linen","apron"],"exclude":["ceramic mug"],"fields":["title"],"fold":true}]}' --control-ids 1000000001 --agent
etsy-pp-cli market price --group-by class --rules-json '{"targets":[{"name":"linen apron","all":["linen","apron"],"exclude":["ceramic mug"],"fields":["title"],"fold":true}]}' --agent
```

### Compare two snapshots

Capture the same synthetic terms twice at different times, keeping the returned snapshot IDs. Substitute those IDs for 1 and 2:

```bash
printf '%s\n' 'linen apron' 'ceramic mug' > terms.txt
etsy-pp-cli market snapshot --terms terms.txt --per-page 25 --pages 1 --max-calls 100
etsy-pp-cli market trend --from 1 --to 2 --agent
```

### Analytics over synced data

```bash
etsy-pp-cli sync --max-calls 100 --agent
etsy-pp-cli analytics --type listings --group-by state --json
```

If sync stops at the cap, rerun it to finish; analytics reports only the data already stored.

### Erase a buyer on request

Preview, apply, then confirm the remaining selection as JSON. Stdin accepts one email per line and avoids putting the address in command arguments:

```bash
printf '%s\n' 'buyer@example.invalid' | etsy-pp-cli local-data erase-buyer --buyer-email -
printf '%s\n' 'buyer@example.invalid' | etsy-pp-cli local-data erase-buyer --buyer-email - --yes
printf '%s\n' 'buyer@example.invalid' | etsy-pp-cli local-data erase-buyer --buyer-email - --json
```

Read `kept_unlinkable_reviews` and the vacuum status: a zero-match preview does not prove exports or backups were erased. Buyer suppression blocks older data from returning on sync; removing suppression permits re-sync subject to retention floors.

### Keep only 90 days of receipts

```bash
etsy-pp-cli local-data prune --older-than 2160h --receipts
etsy-pp-cli local-data prune --older-than 2160h --receipts --yes --vacuum
```

Only old shipped receipts and their buyer dependents are selected. Reviews remain. Later sync rejects never-stored old shipped receipts and remembered receipt dependents; stored updates still pass. The persisted floor is a cutoff from pruning, so rerun prune periodically to maintain a moving 90-day window. Preview and apply `--clear-floor --receipts` to re-admit previously pruned receipts when no suppression applies.

### MCP server for a busy agent

```bash
PP_MCP_MAX_BYTES=60000 PP_MCP_MAX_CONCURRENCY=4 PP_MCP_QUEUE_TIMEOUT=20s etsy-pp-mcp
```

Follow paging cursors with identical arguments; on a busy error, retry later or reduce parallel calls. Stdio retains its five-worker limit. See [MCP limits](#mcp-limits) for the separate local-read cap.


### Daily shop check for an agent

```bash
etsy-pp-cli shop attention --agent
```

One local-store summary instead of many live calls.

### Fees by type last month

```bash
etsy-pp-cli money fees --since 30d --agent
```

Groups synced ledger entries by fee type.

### GPSR readiness before the deadline

```bash
etsy-pp-cli listings audit --rules gpsr --agent --select results.listing_id,results.title,results.findings
```

Lists only the listings and missing GPSR fields. Shop commands wrap rows as {results, notes}, so --select paths start with results.

### Renewal planning

```bash
etsy-pp-cli listings expiring --within 30d
```

Soonest-expiring active listings and their auto-renew setting.

### Market snapshot then demand

```bash
etsy-pp-cli market snapshot --terms terms.txt --pages 1 --per-page 25 --with-reviews --max-calls 200
```

Capture public search results with review dates once, then run market demand, shops, tags and price offline (market gaps also needs a --rules file).

### Audit, then preview a bulk fix

```bash
etsy-pp-cli listings bulk-edit --listings 1000000001,1000000002 --field tags --add "linen apron"
```

Preview-only: shows every proposed change. For a large fix, run `listings audit --plan-out plan.json` (add --force to overwrite) and preview it with `listings bulk-edit --plan plan.json`. Nothing is sent until --apply --yes (needs auth login --all-scopes); `--resume JOB` continues an interrupted job.

## Budgets, resuming and privacy

`sync --max-calls N` and `market snapshot --max-calls N` charge every API request attempt, including retries. Cache hits before a sync stop are uncharged; after a stop, sync refuses even cache hits. OAuth refresh POSTs and followed redirects are not separately charged. A snapshot call cap, including one reached inside a retry, saves status `partial` and exits 0 with a resume note.

Receipt payment 403s retry next sync. A run with both ok checks and 403s warns and clears `ever_ok` on its forbidden receipts. With no ok checks, a 403 on an `ever_ok` receipt triggers exactly one verification request on a different, unwalked `ever_ok` receipt, within the call and quota budget. A successful verification records ok and clears `ever_ok` on this run's forbidden receipts; a forbidden or unavailable verification preserves history and exits 1 with the `transactions_r` scope hint. No prior successful check also exits 1. `--strict` keeps its strict reason and includes the scope hint when applicable. Capped runs exit 7, except zero-progress runs with a scope problem, which exit 1.

Quota observations are stored privately in `<StateDir>/etsy-quota-<hash12(keystring)>.json` (SHA-256, first 12 hex characters), shared by OAuth sessions using the same API keystring and separated across keys. The legacy `etsy-quota.json` is ignored; with no API key, quota observation and loading do nothing. Writers merge under a lock for at most 100 ms (or the request deadline), then fall back to process memory if busy. The newest observation supplies the whole record; balances within five seconds use the smaller remainder. Readers use atomic snapshots. State files use mode 0600 and a newly created state directory uses 0700. There is no CLI-side rolling budget: Etsy’s QPD is a rolling 24-hour window that the CLI only observes, so remaining quota can rise and there is no promised midnight reset.

Resume after the window rolls when quota is exhausted. `doctor` reports the selected key’s observed quota balance and age, or unknown when no observation exists in the last 24h.

```bash
etsy-pp-cli doctor --json
etsy-pp-cli sync --max-calls 100 --agent
```


- Daily budget is 5,000 Etsy calls. `sync --max-calls N` hard-caps one run; `sync --ignore-budget` skips only the preflight estimate check.
- `market snapshot --per-page N` (1..100) sets listings per page; each page costs up to 1 + N calls. The summary's `reviews_capped` counts listings whose reviews were truncated at the page cap.
- `listings bulk-edit --resume JOB` continues an interrupted job (`--restart` is deprecated; combining it with `--resume` exits 2); `--flatten-variation-prices` is required to overwrite all variation prices with one price; nothing is sent without `--apply --yes`.
- `listings audit --plan-out FILE` refuses to overwrite an existing file unless `--force`, and never follows symlinks.
- Over MCP, hand-written write commands are preview-only (`--yes` is never forwarded); endpoint writes go through `etsy_execute` with `confirm:true`. `--ignore-budget` is refused and `--max-calls`/`--pages` are clamped.
- MCP buyer data is an allowlist (fail closed): `sql`/`search` expose only non-buyer resource types, `entity_lookups` is filtered to the same allowlist, `orders unshipped --full`, `reviews digest --text` and `analytics --type <buyer type>` refuse over MCP, and `teach`/`teach-pattern` refuse buyer-bearing resource types while `recall` never reads buyer rows for entities, and `learnings list`/`candidates`, `playbook list` and recall's candidate section hide buyer-bearing learnings and say how many, unless the operator sets `ETSY_MCP_ALLOW_BUYER_PII=1`. A conformance test walks every registered MCP tool against sentinel buyer data to keep this true. A second arm with the opt-in on shows each tool could have leaked: it emits a sentinel, or it is listed as a non-buyer reader with a reason naming what it reads (checked against the source). The seed holds sentinels in the learn tables too, so a learnings, candidates or playbook tool cannot be listed away. Live MCP endpoint tools return raw API data (needs operator OAuth `transactions_r`). With the opt-in, cursor paging can return the full result of `orders_unshipped` with `full:true`; `limit` still caps rows. Over-budget `reviews_digest` with `text:true` returns a non-pageable preview; it has no `limit` parameter.

## Auth Setup

Every request sends `x-api-key: <keystring>:<shared_secret>` (required by Etsy since 2026-02-09): set ETSY_API_KEY to your keystring and ETSY_SHARED_SECRET to your shared secret from etsy.com/developers/your-apps, or put both in ETSY_API_KEY as `keystring:secret`. `etsy-pp-cli app ping` confirms Etsy accepts the key. Public data (active listings, shops, reviews, taxonomy, market snapshots) needs only that header. Shop data (orders, money, drafts, inventory) also needs `etsy-pp-cli auth login` (OAuth 2.0 PKCE) and ETSY_SHOP_ID set to your shop id. The default login is read-only (listings_r shops_r transactions_r); `auth login --all-scopes` adds email_r, write, delete, address and profile scopes. If your app registers an https redirect, use the two-step flow: `auth login --redirect-uri <url>` prints the authorize URL, then `auth login --callback-url '<redirected url>'` finishes it. Access tokens last 1 hour and refresh automatically for 90 days.

Run `etsy-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array). Nested market reports (tags, trend) refuse these formats; use --json or --agent. `--csv` prefixes `'` on a cell or header key that starts with `=`, `+`, `-`, `@`, tab, CR, or LF (fullwidth `＝＋－＠` too) so a spreadsheet does not run it as a formula; a plain signed number such as `-5` or `+2.5e3` is unchanged.
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  etsy-pp-cli listings get 1000000001 --agent --select listing_id,title,state
  ```
- **Previewable** — `--dry-run` shows the request without sending. A completed dry run exits 0 with `"success": true` and `"dry_run": true` and no HTTP `status`; validation failures still exit non-zero with `"success": false`.
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

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

| Environment variable | Purpose |
| --- | --- |
| `PP_MCP_MAX_BYTES` | All MCP response budgets; default 32000, bounded to 2048–16777216 bytes |
| `PP_MCP_MAX_CONCURRENCY` | Mirror admission cap; default 4, range 1–32 |
| `PP_MCP_QUEUE_TIMEOUT` | Admission wait; default 20s, range 1ms–5m |
| `ETSY_MCP_ALLOW_BUYER_PII` | Operator opt-in for restricted MCP buyer reads; `limit` still caps rows in `orders_unshipped`, while `reviews_digest` previews cannot page |
| `ETSY_NO_LOCAL_DATA` | Disable new local copies with `1` |
| `ETSY_DATA_SOURCE` | Default `auto`, `live` or `local` read source |


Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `ETSY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ETSY_CONFIG_DIR`, `ETSY_DATA_DIR`, `ETSY_STATE_DIR`, `ETSY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ETSY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `etsy-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "etsy": {
        "command": "etsy-pp-mcp",
        "env": {
          "ETSY_HOME": "/srv/etsy"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ETSY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ETSY_HOME`, or `doctor` will not find credentials left under the former root.

Local data is unencrypted and relies on OS account permissions plus disk encryption. Reads in the default `--data-source auto` mode are written to the SQLite store; GET responses are cached for five minutes under the cache dir. `local-data purge` previews what it would erase (store files, response cache, learn journal) and deletes only with `--yes`; it is not exposed over MCP. `ETSY_NO_LOCAL_DATA=1` disables the response cache and store writes, turns `auto` reads into live reads, and blocks `sync` and `workflow archive`; `--data-source local` still reads an existing store, and `ETSY_DATA_SOURCE` sets the `--data-source` default.

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
etsy-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "etsy-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `etsy-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `etsy-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `etsy-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
etsy-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
etsy-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
etsy-pp-cli teach-playbook \
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
etsy-pp-cli playbook amend \
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

`etsy-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `ETSY_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
etsy-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
etsy-pp-cli feedback --stdin < notes.txt
etsy-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ETSY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ETSY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
etsy-pp-cli profile save briefing --json
etsy-pp-cli --profile briefing listings get 1000000001
etsy-pp-cli profile list --json
etsy-pp-cli profile show briefing
etsy-pp-cli profile delete briefing --dry-run
etsy-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error: unknown analytics type/field, invalid snapshot IDs or rule keys, nested tags/trend CSV/plain, combined bulk-edit restart/resume, missing write confirmation |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 6 | Partial failure |
| 7 | Quota or call cap reached; sync progress is retained. Resume after the window rolls for quota stops, or rerun with an adequate call cap |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `etsy-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/etsy/cmd/etsy-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add etsy-pp-mcp -- etsy-pp-mcp
   ```
3. Verify: `claude mcp list`

For a remote host, `etsy-pp-mcp --transport http` requires `ETSY_MCP_HTTP_TOKEN` set to at least 32 characters (callers send `Authorization: Bearer <token>`; `openssl rand -hex 32` prints 64 hex characters and meets that minimum), binds `127.0.0.1:7777` by default, and needs `--tls-cert`/`--tls-key` for any non-loopback `--addr`. Requests with no `Origin` header are accepted. Loopback origins are accepted; any other `Origin` is refused unless listed in `ETSY_MCP_HTTP_ALLOWED_ORIGINS` (comma-separated `scheme://host[:port]` values). `ETSY_MCP_READ_ONLY=1` drops mirrored write commands (local-write commands that only touch the CLI's own learn store stay) and makes the raw execute tool refuse state-changing calls.

### Paging MCP results

For mirror tools, check top-level `truncated`: when absent, the output is complete. With `agent: true`, paging fields stay at the top level beside `meta` and `results`. Read `_pp_note` if present, else `note`; with `agent: true` the guidance is always top-level `note`. Typed endpoint tools, the `etsy_*` code-orchestration tools, `search` and `context` signal truncation with `_pp_truncated` (and `truncated` on previews and paged lists); `sql` signals it with `truncated: true` and a note. Treat EITHER marker as incomplete. Typed local pages resume via `_pp_cursor`, not `cursor`.

- The read-only local report tools that list a `cursor` parameter in their schema (named in the CHANGELOG) accept `cursor`. When `next_cursor` is present, resend the SAME arguments plus `cursor: next_cursor`, including inline content; each page re-runs the command. A supplied cursor returns an envelope even on the final page. Tools whose result is a single object (`market_tags`, `market_trend`, `money_reconcile`, `reviews_digest`, `shop_attention`) accept `cursor` but never page; over budget they return a non-pageable Preview. `money_summary` does not accept `cursor`: its wall-clock `since`/`until` would always make it stale. Tools such as `market_snapshot` and `listings_bulk_edit` have no cursor; follow their over-budget note and do not re-run for the rest.
- Errors start with `cursor_invalid`, `cursor_wrong_tool`, `cursor_args_mismatch`, or `cursor_stale`. Stale means results changed since page 1: a store write or time-dependent fields (including a day boundary). Re-run from the first page without `cursor`. For `market_demand`, `market_gaps`, `market_price`, `market_shops` and `market_tags`, a new snapshot can also cause staleness; to keep pages consistent, pass `snapshot` set to the page-1 `snapshot_id` (top-level, or `results.snapshot_id` with `agent: true`; if `select` dropped it, re-run once without `select` to read it).
- `PP_MCP_MAX_BYTES` sets the response budget (default 32000 bytes, floor 2048, ceiling 16 MiB / 16777216 bytes; an unparsable, zero or negative value warns and uses 32000; a leading + is accepted). Stderr capture has a fixed 64 KiB cap. Use `compact:true` for smaller market rows. A `preview` is a string fragment, never JSON; it cannot be paged and carries no `next_cursor`.
- Over MCP, use inline `--rules-json` for `market gaps` and `market price` (price requires `--group-by class`), and `--control-ids` for gaps. Each accepts up to 96 KiB; on Windows the whole command must fit about 31 KiB. All rule sources allow at most 256 patterns of at most 512 bytes each. On unix, each argument is checked at 120 KiB, and combined argv+env must still fit the OS limit (about 1 MiB on macOS).

## Direct Use

1. Check if installed: `which etsy-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   etsy-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `etsy-pp-cli <command> --help`.

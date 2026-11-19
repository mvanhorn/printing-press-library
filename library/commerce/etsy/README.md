# Etsy CLI

**Etsy Open API v3 commands plus a local SQLite store for offline fee reports, ledger reconciliation, listing and GPSR audits, market research, and previewed bulk edits.**

Log in, set ETSY_SHOP_ID, run `etsy-pp-cli sync`, then answer fulfilment, money and catalog questions offline with `money fees`, `money reconcile`, `listings audit --rules gpsr`, `listings expiring`, `inventory audit` and `orders unshipped`. Public market research (`market snapshot`, then `market shops`, `gaps`, `tags`, `price`) needs only the API key. The default login is read-only; writes need `auth login --all-scopes` and an explicit `--yes`, and `listings bulk-edit` previews every change until `--apply --yes`.

## Install

The recommended path installs both the `etsy-pp-cli` binary and the `pp-etsy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install etsy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install etsy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install etsy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install etsy --agent claude-code
npx -y @mvanhorn/printing-press-library install etsy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.11 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/etsy/cmd/etsy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/etsy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install etsy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-etsy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-etsy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install etsy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/etsy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ETSY_API_KEY` (your app keystring) and `ETSY_SHARED_SECRET` when Claude Desktop prompts you. `ETSY_SHOP_ID` and `ETSY_OAUTH2` are optional: set the shop id for shop-scoped data, and paste an OAuth bearer token only if you do not use `etsy-pp-cli auth login`.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/etsy/cmd/etsy-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "etsy": {
      "command": "etsy-pp-mcp",
      "env": {
        "ETSY_API_KEY": "your-keystring",
        "ETSY_SHARED_SECRET": "your-shared-secret",
        "ETSY_SHOP_ID": "your-numeric-shop-id"
      }
    }
  }
}
```

`ETSY_API_KEY` and `ETSY_SHARED_SECRET` are required (or put `keystring:secret` in `ETSY_API_KEY` alone). `ETSY_SHOP_ID` is only needed for shop-scoped data. `ETSY_OAUTH2` is an optional pasted bearer token; `etsy-pp-cli auth login` normally supplies it.

</details>

### Remote MCP over HTTP

`etsy-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `ETSY_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `ETSY_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export ETSY_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
etsy-pp-mcp --transport http
```

<!-- etsy-ext: MCP local writes, budget ceilings and buyer privacy contract. -->
Set `ETSY_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Local-write commands, including `market snapshot`, are excluded. Offline `market gaps` remains available.

### Paging MCP results

For mirror tools, check top-level `truncated`: when absent, the output is complete. With `agent: true`, paging fields stay at the top level beside `meta` and `results`. Read `_pp_note` if present, else `note`; with `agent: true` the guidance is always top-level `note`. Typed endpoint tools, the `etsy_*` code-orchestration tools, `search` and `context` signal truncation with `_pp_truncated` (and `truncated` on previews and paged lists); `sql` signals it with `truncated: true` and a note. Treat EITHER marker as incomplete. Typed local pages resume via `_pp_cursor`, not `cursor`.

- The read-only local report tools that list a `cursor` parameter in their schema (named in the CHANGELOG) accept `cursor`. When `next_cursor` is present, resend the SAME arguments plus `cursor: next_cursor`, including inline content; each page re-runs the command. A supplied cursor returns an envelope even on the final page. Tools whose result is a single object (`market_tags`, `market_trend`, `money_reconcile`, `reviews_digest`, `shop_attention`) accept `cursor` but never page; over budget they return a non-pageable Preview. `money_summary` does not accept `cursor`: its wall-clock `since`/`until` would always make it stale. Tools such as `market_snapshot` and `listings_bulk_edit` have no cursor; follow their over-budget note and do not re-run for the rest.
- Errors start with `cursor_invalid`, `cursor_wrong_tool`, `cursor_args_mismatch`, or `cursor_stale`. Stale means results changed since page 1: a store write or time-dependent fields (including a day boundary). Re-run from the first page without `cursor`. For `market_demand`, `market_gaps`, `market_price`, `market_shops` and `market_tags`, a new snapshot can also cause staleness; to keep pages consistent, pass `snapshot` set to the page-1 `snapshot_id` (top-level, or `results.snapshot_id` with `agent: true`; if `select` dropped it, re-run once without `select` to read it).
- `PP_MCP_MAX_BYTES` sets the response budget (default 32000 bytes, floor 2048, ceiling 16 MiB / 16777216 bytes; an unparsable, zero or negative value warns and uses 32000; a leading + is accepted). Stderr capture has a fixed 64 KiB cap. Use `compact:true` for smaller market rows. A `preview` is a string fragment, never JSON; it cannot be paged and carries no `next_cursor`.
- Over MCP, use inline `--rules-json` for `market gaps` and `market price` (price requires `--group-by class`), and `--control-ids` for gaps. Each accepts up to 96 KiB; on Windows the whole command must fit about 31 KiB. All rule sources allow at most 256 patterns of at most 512 bytes each. On unix, each argument is checked at 120 KiB, and combined argv+env must still fit the OS limit (about 1 MiB on macOS).

MCP children reject `--ignore-budget`. Sync is capped at 500 calls and 50 pages per resource; market snapshots are capped at 500 calls and three pages per term, including resumed snapshots. Lower caps remain effective.

MCP `sql` and `search` treat buyer data as an allowlist (fail closed): only resource types classified as non-buyer (listings, shops, inventory, images, shipping profiles, taxonomies, ledger entries and similar) are visible; every other type, including any new one, is hidden. `sql` sees a connection-local view restricted to the allowlist and refuses direct reads of buyer tables (receipts, transactions, user addresses, payments, payment checks (`etsy_payment_checks`), users, reviews, economic operators) and the learn tables (`search_learnings`, `learn_candidates`, `learning_playbooks`, `search_patterns`, `learn_recall_misses`), FTS, schema-qualified and schema-introspection queries; `search` applies the allowlist inside its SQL (so `limit` counts only visible rows) and always reports `buyer_pii: "filtered"` with the same note whether or not a buyer record matched. The derived `entity_lookups` table is covered the same way: `sync` extracts lookups only from allowlisted types and purges synced rows of any other kind on every refresh, and `sql` sees `entity_lookups` through a kind-filtered view. Over MCP, `orders unshipped --full`, `reviews digest --text` and `analytics --type <buyer type>` also refuse. `teach` and `teach-pattern` refuse a buyer-bearing `--resource-type` over MCP, and `recall` never reads buyer rows to extract entities, so a taught receipt cannot be dereferenced back into buyer names. Over MCP without the opt-in, `learnings list` (rows and `--warnings`), `learnings candidates`, `playbook list` and `recall`'s candidate section drop buyer-bearing learnings (a taught `resource_type` that is not on the allowlist, or a candidate or playbook family sharing a token with one) and report how many were hidden (in this restricted mode they always emit the object form `{<items>, buyer_hidden, note}`, with `buyer_hidden` 0 when nothing was hidden; outside it they emit a bare array); `learnings confirm` and `learnings reject` refuse a buyer-bearing candidate. A conformance test (`internal/mcpconformance`) walks every tool the MCP server actually registers, in both read-only and read-write registration, against a store seeded with buyer sentinels (including CLI-taught buyer learnings, candidates, playbooks and teach-log warnings), and fails if any sentinel reaches any tool output without the opt-in; it also fails if a registered tool is neither exercised nor listed in its skip list with a reason. A second arm re-walks every non-skipped tool with the opt-in on and fails unless each tool emits a buyer sentinel or is listed in the test's `etsyNonBuyerReading` map with a source-checked reason naming what the tool reads (and fails on a stale entry), so a clean default arm cannot be vacuous for a tool that never reads buyer data in that walk. Tools that only talk to the live API (including `export` and `sync`) are skipped there. All of this lifts only when the operator sets `ETSY_MCP_ALLOW_BUYER_PII=1` (also `true` or `yes`). Live MCP endpoint tools (the generated per-endpoint tools) are not covered: they return the raw API response, which includes buyer data when the operator's OAuth token has the `transactions_r` scope. Set `ETSY_MCP_ALLOW_BUYER_PII=1` only when raw buyer data is needed. These guards apply to the MCP SQL/search tools and the CLI commands above. With the opt-in, cursor paging can return the full result of `orders_unshipped` with `full:true`; `limit` still caps rows. Over-budget `reviews_digest` with `text:true` returns a non-pageable preview; it has no `limit` parameter.

`listings audit --plan-out FILE` creates a private 0600 file and refuses an existing destination unless `--force` is passed. Symlinks and other non-regular existing destinations are always refused.

## Authentication

Every request sends `x-api-key: <keystring>:<shared_secret>` (required by Etsy since 2026-02-09): set ETSY_API_KEY to your keystring and ETSY_SHARED_SECRET to your shared secret from etsy.com/developers/your-apps, or put both in ETSY_API_KEY as `keystring:secret`. `etsy-pp-cli app ping` confirms Etsy accepts the key. Public data (active listings, shops, reviews, taxonomy, market snapshots) needs only that header. Shop data (orders, money, drafts, inventory) also needs `etsy-pp-cli auth login` (OAuth 2.0 PKCE) and ETSY_SHOP_ID set to your shop id. The default login is read-only (listings_r shops_r transactions_r); `auth login --all-scopes` adds email_r, write, delete, address and profile scopes. If your app registers an https redirect, use the two-step flow: `auth login --redirect-uri <url>` prints the authorize URL, then `auth login --callback-url '<redirected url>'` finishes it. Access tokens last 1 hour and refresh automatically for 90 days.

## Quick Start

```bash
# Checks config and credentials without calling the network.
etsy-pp-cli doctor --dry-run

# Confirms Etsy accepts your keystring:shared_secret (a 403 means a missing or mismatched secret, or an inactive app).
etsy-pp-cli app ping

# Reports the key format, the rolling 24h quota balance, and whether an OAuth session is present.
etsy-pp-cli doctor

# After auth login and ETSY_SHOP_ID, mirrors your shop once so every report after this is free.
etsy-pp-cli sync --since 30d

# One summary of what needs attention today.
etsy-pp-cli shop attention --agent

```

## Unique Features

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
printf '%s\n' 'buyer@example.invalid' | etsy-pp-cli local-data erase-buyer --buyer-email -
printf '%s\n' 'buyer@example.invalid' | etsy-pp-cli local-data erase-buyer --buyer-email - --yes
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

## Usage

Run `etsy-pp-cli --help` for the full command reference and flag list.

## Budgets, resuming and privacy

`sync --max-calls N` and `market snapshot --max-calls N` charge every API request attempt, including retries. Cache hits before a sync stop are uncharged; after a stop, sync refuses even cache hits. OAuth refresh POSTs and followed redirects are not separately charged. A snapshot call cap, including one reached inside a retry, saves status `partial` and exits 0 with a resume note.

Receipt payment 403s retry next sync. A run with both ok checks and 403s warns and clears `ever_ok` on its forbidden receipts. With no ok checks, a 403 on an `ever_ok` receipt triggers exactly one verification request on a different, unwalked `ever_ok` receipt, within the call and quota budget. A successful verification records ok and clears `ever_ok` on this run's forbidden receipts; a forbidden or unavailable verification preserves history and exits 1 with the `transactions_r` scope hint. No prior successful check also exits 1. `--strict` keeps its strict reason and includes the scope hint when applicable. Capped runs exit 7, except zero-progress runs with a scope problem, which exit 1.

Quota observations are stored privately in `<StateDir>/etsy-quota-<hash12(keystring)>.json` (SHA-256, first 12 hex characters), shared by OAuth sessions using the same API keystring and separated across keys. The legacy `etsy-quota.json` is ignored; with no API key, quota observation and loading do nothing. Writers merge under a lock for at most 100 ms (or the request deadline), then fall back to process memory if busy. The newest observation supplies the whole record; balances within five seconds use the smaller remainder. Readers use atomic snapshots. State files use mode 0600 and a newly created state directory uses 0700. There is no CLI-side rolling budget: Etsy’s QPD is a rolling 24-hour window that the CLI only observes, so remaining quota can rise and there is no promised midnight reset.

Resume after the window rolls when quota is exhausted. `doctor` reports the selected key’s observed quota balance and age, or unknown when no observation exists in the last 24h.

```bash
etsy-pp-cli doctor --json
etsy-pp-cli sync --max-calls 100 --agent
```


<!-- etsy-ext: round-2 flag reference -->
- **Daily budget.** Etsy allows 5,000 calls a day. `sync` checks its estimated call count against the remaining budget first. `sync --max-calls N` hard-caps live calls for one run; `sync --ignore-budget` skips only the preflight check, never the hard cap or a daily-quota stop.
- **Budget re-check before fan-out.** `sync` checks the budget again after the flat resources finish and before the per-receipt dependents, using the receipt counts stored at that moment. Other dependents still refuse (exit code 7, partial data stays stored) when they would not fit. The payments dependent instead runs capped at what is left (the remaining `--max-calls`, or the known remaining daily quota minus a reserve of 5% or the smallest balance the budget gate accepts, whichever is larger; below that floor the run refuses instead of walking payments). When no Etsy response has been seen yet (quota unknown), the preflight counts only the flat-phase calls; if a run still plans more than 1,000 calls once the dependents are known, `sync` refuses and asks for `--max-calls N` or `--ignore-budget`.
- **Incremental payments.** The default sync fetches `/payments` only for receipts that have never been checked or whose `updated_timestamp` is newer than when they were last checked; `sync --full` walks every receipt (never-checked receipts first, then the oldest-checked, so repeated capped `--full` runs cover every receipt). A successful `/payments` response (even an empty one) records a per-receipt check, so an interrupted walk resumes where it stopped and a refund that lands during a walk is picked up next run. A second sync with no receipt changes makes no payments requests.
- **Exit codes.** Stops exit 7: a daily-quota stop, a `--max-calls` stop (flat phase or during the dependents), the payments reserve refusal (`payments: daily reserve reached (N left, floor F); ...`, zero `/payments` requests, attempt not marked complete) and a capped payments run. Failures exit non-7: critical failures, `--strict` failures (run stops excluded), all resources failing, a capped payments walk where every payment call failed (`payments_failed` event), and payment scope failures (exit 1 when a forbidden receipt previously succeeded or no payment check has ever succeeded; other forbidden receipts warn with success history). Only payments failures change the payments exit; another resource's failure does not. With `--json`/`--compact`, the machine summary on stdout (`total_records` and the other counts) is printed once before any exit-7 return after the sync started, so MCP callers keep it; it covers resources that finished and excludes the resource a stop interrupted. The `--max-calls` stop message says to re-run sync and whether a re-run makes progress (a flat-phase stop restarts capped listings walks; a dependents-phase stop resumes where it stopped). The reserve refusal also suggests narrowing `--resources` to skip `shops_receipts_payments`. Failed or strict runs still refresh derived lookups before exiting.
- **Capped payments progress.** Payments progress is saved per receipt. When the allowance is smaller than the receipts still to check, the run checks as many as fit, prints `payments: N of M receipts checked; re-run to continue (progress is saved)` (a `payments_capped` sync event in JSON mode), and exits 7; the next run continues with the remaining receipts. A daily-quota or `--max-calls` stop from the Etsy client keeps priority over this message, and so do a placeholder credential and critical failures (`--strict` included). A capped run that checked nothing, or whose payments walk failed, reports that failure instead of claiming progress was saved.
- **Reviews in the default sync.** `sync` now includes `reviews` (incremental through `min_created`), so `reviews digest` has data after a plain `etsy-pp-cli sync`. Reviews carry buyer-written text and are stored locally like receipts.
- **Market snapshots.** `market snapshot --per-page N` (1..100, default 100) sets listings per search page, and each page costs up to 1 + N calls, so `--per-page 25` is a cheaper scan. When some listings have more reviews than the per-listing page cap, the run summary reports `reviews_capped` (how many listings were truncated) and demand figures for them are a floor.
- **Bulk edits.** `listings bulk-edit --resume JOB` continues an interrupted job; `--restart` is deprecated (a fresh run is now the default); combining `--restart` and `--resume` exits 2. `--flatten-variation-prices` is required to replace every variation price of a listing with one price. Nothing is sent without `--apply --yes`.
- **Plan files.** `listings audit --plan-out FILE` creates the file with mode 0600 and refuses to overwrite an existing file unless you also pass `--force`. It never follows symlinks.
- **MCP.** Hand-written write commands (`listings bulk-edit`, `listings copy`, `inventory set-qty`, ...) are preview-only over MCP: `--yes` is never forwarded. The MCP write path for API endpoints is `etsy_execute` with `confirm:true`. MCP callers cannot use `--ignore-budget`, and `--max-calls`/`--pages` are clamped (500 calls, 3 snapshot pages, 50 sync pages).
- **Buyer data over MCP.** See [Paging MCP results](#paging-mcp-results) for the allowlist, operator opt-in and live endpoint limits.

## Paths & environment variables


| Environment variable | Purpose |
| --- | --- |
| `PP_MCP_MAX_BYTES` | All MCP response budgets; default 32000, bounded to 2048–16777216 bytes |
| `PP_MCP_MAX_CONCURRENCY` | Mirror admission cap; default 4, range 1–32 |
| `PP_MCP_QUEUE_TIMEOUT` | Admission wait; default 20s, range 1ms–5m |
| `ETSY_MCP_ALLOW_BUYER_PII` | Operator opt-in for restricted MCP buyer reads; `limit` still caps rows in `orders_unshipped`, while `reviews_digest` previews cannot page |
| `ETSY_NO_LOCAL_DATA` | Disable new local copies with `1` |
| `ETSY_DATA_SOURCE` | Default `auto`, `live` or `local` read source |

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ETSY_CONFIG_DIR`, `ETSY_DATA_DIR`, `ETSY_STATE_DIR`, or `ETSY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ETSY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ETSY_HOME=/srv/etsy
etsy-pp-cli doctor
```

Under `ETSY_HOME=/srv/etsy`, the four dirs resolve to `/srv/etsy/config`, `/srv/etsy/data`, `/srv/etsy/state`, and `/srv/etsy/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:ETSY_HOME = "C:\srv\etsy"
etsy-pp-cli doctor
```

Under `$env:ETSY_HOME = "C:\srv\etsy"`, the four dirs resolve to `C:\srv\etsy\config`, `C:\srv\etsy\data`, `C:\srv\etsy\state`, and `C:\srv\etsy\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `ETSY_DATA_DIR` overrides an explicit `--home` for that kind. Use `ETSY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ETSY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `etsy-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

For selective cleanup and persisted sync policies, see [Local data and privacy](#local-data-and-privacy).

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `etsy-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `etsy-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `ETSY_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `ETSY_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### app

Manage app

- **`etsy-pp-cli app ping`** - Check to confirm connectivity to the Etsy API with an application
- **`etsy-pp-cli app token-scopes`** - Check the scopes of the provided token

### buyer_taxonomy

Manage buyer taxonomy

- **`etsy-pp-cli buyer-taxonomy get-nodes`** - Retrieves the full hierarchy tree of buyer taxonomy nodes.
- **`etsy-pp-cli buyer-taxonomy get-properties-by-id`** - Retrieves a list of product properties, with applicable scales and values, supported for a specific buyer taxonomy ID.

### economic_operators

Manage economic operators

- **`etsy-pp-cli economic-operators create-shop`** - Creates a new economic operator (a manufacturer or EU responsible person record) for a specific shop, used for GPSR compliance on listings. `operator_name`, `email`, `first_line`, `city`, `zip`, `country_iso`, and `phone` are required and cannot be blank; `trademark_name`, `second_line`, and `phone_country_iso` are optional. `state` is required if the `country_iso` requires a state (for example `US`, `CA`, `AU`, `IN`, `BR`, `MX`, `ES`, `IT`); it is optional for every other country. See the [full list of countries](/documentation/tutorials/gpsr-economic-operators#countries-requiring-a-state). The response contains the `economic_operator_id` to pass as `gpsr_manufacturer_id` or `gpsr_responsible_person_id` when creating or updating a [listing](/documentation/reference#tag/ShopListing). A manufacturer can be located anywhere, but an operator used as an EU responsible person must have an address in the European Union; this is checked when a listing references the operator. There is no delete endpoint: use [updateShopEconomicOperator](/documentation/reference#operation/updateShopEconomicOperator) to correct a record. See the [GPSR Economic Operators Tutorial](/documentation/tutorials/gpsr-economic-operators) for the full guide, including eligibility and the enforcement timeline.
- **`etsy-pp-cli economic-operators get-shop`** - Retrieves all economic operators (manufacturer and EU responsible person records) of a specific shop, used for GPSR compliance on listings. Deleted operators are not included. Use the `economic_operator_id` of an entry as `gpsr_manufacturer_id` or `gpsr_responsible_person_id` on a [listing](/documentation/reference#tag/ShopListing). See the [GPSR Economic Operators Tutorial](/documentation/tutorials/gpsr-economic-operators) for the full guide.
- **`etsy-pp-cli economic-operators get-shop-shops`** - Retrieves a single economic operator, referenced by economic operator ID and shop ID. Returns `404` if the operator does not exist in the shop or has been deleted. See the [GPSR Economic Operators Tutorial](/documentation/tutorials/gpsr-economic-operators) for the full guide.
- **`etsy-pp-cli economic-operators update-shop`** - Updates an existing economic operator for a specific shop. Every field is optional and only the fields you send are changed. Send `null` for `trademark_name`, `second_line`, `state`, or `phone_country_iso` to clear it. `operator_name` can be at most 50 characters. `state` is required for [some countries](/documentation/tutorials/gpsr-economic-operators#countries-requiring-a-state). The change applies to every [listing](/documentation/reference#tag/ShopListing) that references this operator. Returns `404` if the economic operator does not exist in the shop. See the [GPSR Economic Operators Tutorial](/documentation/tutorials/gpsr-economic-operators) for the full guide.

### holiday_preferences

Manage holiday preferences

- **`etsy-pp-cli holiday-preferences get`** - Retrieves a list of holidays that are available to a shop to set a preference for. Currently only supported in the US and CA
- **`etsy-pp-cli holiday-preferences update`** - Updates the preference for whether the seller will process orders or not on the holiday. Currently only supported in the US and CA

### ledger_entries

Manage ledger entries

- **`etsy-pp-cli ledger-entries get-shop-payment-account`** - Get a Shop Payment Account Ledger's Entries
- **`etsy-pp-cli ledger-entries get-shop-payment-account-ledger-entry`** - Get a single Shop Payment Account Ledger's Entry

### listing_files

Manage listing files

- **`etsy-pp-cli listing-files delete`** - Deletes a file from a specific listing. When you delete the final file for a digital listing, the listing converts into a physical listing. The response to a delete request returns a list of the remaining file records associated with the given listing.
- **`etsy-pp-cli listing-files get`** - Retrieves a single file associated with the given digital listing. Requesting a file from a physical listing returns an empty result.
- **`etsy-pp-cli listing-files get-all`** - Retrieves all the files associated with the given digital listing. Requesting files from a physical listing returns an empty result.
- **`etsy-pp-cli listing-files upload`** - Uploads a new file for a digital listing, or associates an existing file with a specific listing. You must either provide the `listing_file_id` of an existing file, or the name and binary file data for a file to upload. Associating an existing file to a physical listing converts the physical listing into a digital listing, which removes all shipping costs and any product and inventory variations.

### listing_images

Manage listing images

- **`etsy-pp-cli listing-images delete`** - Open API V3 endpoint to delete a listing image. A copy of the file remains on our servers, and so a deleted image may be re-associated with the listing without re-uploading the original image; see uploadListingImage.
- **`etsy-pp-cli listing-images get`** - Retrieves all listing image resources for a listing with a specific listing ID.
- **`etsy-pp-cli listing-images get-listings`** - Retrieves the references and metadata for a listing image with a specific image ID.
- **`etsy-pp-cli listing-images upload`** - Uploads or assigns an image to a listing identified by a shop ID with a listing ID. To upload a new image, set the image file as the value for the `image` parameter. You can assign a previously deleted image to a listing using the deleted image's image ID in the `listing_image_id` parameter. When a request contains both `image` and `listing_image_id` parameter values, the endpoint uploads the image in the `image` parameter only. Note: When uploading a new image, data such as colors and size may return as null values due to asynchronous processing of the image. Use getListingImage endpoint to fetch these values.

### listing_inventory

Manage listing inventory

- **`etsy-pp-cli listing-inventory get`** - Retrieves the inventory record for a listing. Listings you did not edit using the Etsy.com inventory tools have no inventory records. Only the owner of the listing may retrieve its inventory record.
- **`etsy-pp-cli listing-inventory get-listing-product`** - Open API V3 endpoint to retrieve a ListingProduct by ID.
- **`etsy-pp-cli listing-inventory get-listings-inventory-by-listing-ids`** - Retrieves the inventory record for each listing referenced by listing ID. Requires the `listings_r` OAuth scope. Limit 100 listing IDs per request. All requested listing IDs must exist — if any single ID is not found, the entire request returns a 404. SKUs within product records are only returned for listings owned by the authenticated user; they are stripped (returned as empty string) for listings owned by other sellers.
- **`etsy-pp-cli listing-inventory update`** - Updates the inventory for a listing identified by a listing ID. The update fails if the supplied values for product sku, offering quantity, price, and/or processing profile are incompatible with values in `*_on_property` fields. When setting a price, assign a float equal to amount divided by divisor as specified in the Money resource.

### listing_personalization

Manage listing personalization

- **`etsy-pp-cli listing-personalization delete`** - Deletes personalization for a listing.
- **`etsy-pp-cli listing-personalization get`** - Retrieves a listing's personalization questions by listing ID.
- **`etsy-pp-cli listing-personalization update`** - Creates or updates personalization settings for a listing, allowing the seller to collect personalization from the buyer. This endpoint will fully replace any existing personalization on the listing.

### listing_translations

Manage listing translations

- **`etsy-pp-cli listing-translations create`** - Creates a ListingTranslation by listing_id and language
- **`etsy-pp-cli listing-translations get`** - Get a Translation for a Listing in the given language
- **`etsy-pp-cli listing-translations update`** - Updates a ListingTranslation by listing_id and language

### listing_variation_images

Manage listing variation images

- **`etsy-pp-cli listing-variation-images get`** - Gets all variation images on a listing.
- **`etsy-pp-cli listing-variation-images update-variation-images`** - Creates variation images on a listing. `variation_images` is an array with inputs for the `property_id`, `value_id`, and `image_id` fields. `image_ids` are associated with a `ListingImage` on the listing associated with the provided `listing_id`. `property_id` and `value_id` pairs are associated with a `ListingProduct` on the listing associated with the provided `listing_id`. `variation_images` should not contain any duplicates. `variation_images` does not contain more than one `property_id` as variation images can only be associated on one property. The update overwrites all existing variation images on a listing, so if your request is successful, the variation images on the listing will be exactly those you specify.

### listing_videos

Manage listing videos

- **`etsy-pp-cli listing-videos delete`** - Open API V3 endpoint to delete a listing video. A copy of the video remains on our servers, and so a deleted video may be re-associated with the listing without re-uploading the original video; see uploadListingVideo.
- **`etsy-pp-cli listing-videos get`** - Retrieves all listing video resources for a listing with a specific listing ID.
- **`etsy-pp-cli listing-videos get-listings`** - Retrieves a single video associated with the given listing. Requesting a video from a listing returns an empty result.
- **`etsy-pp-cli listing-videos upload`** - Uploads a new video for a listing, or associates an existing video with a specific listing. You must either provide the `video_id` of an existing video, or the name and binary file data for a video to upload. If providing a `video_id`, the video must already be associated with the same shop as the listing, but it does not need to be currently associated with the listing. It's important to consider there is a rate limiting to perform 10 uploads per listing per day.

### listings

Manage listings

- **`etsy-pp-cli listings create-draft`** - Creates a physical draft [listing](/documentation/reference#tag/ShopListing) product in a shop on the Etsy channel.
- **`etsy-pp-cli listings delete`** - Open API V3 endpoint to delete a ShopListing. A ShopListing can be deleted only if the state is one of the following: SOLD_OUT, DRAFT, EXPIRED, INACTIVE, ACTIVE and is_available or ACTIVE and has seller flags: SUPRESSED (frozen), VACATION, CUSTOM_SHOPS (pattern), SELL_ON_FACEBOOK
- **`etsy-pp-cli listings delete-property`** - Deletes a property for a Listing.
- **`etsy-pp-cli listings find-all-active`** - A list of all active listings on Etsy paginated by their creation date. Without sort_order listings will be returned newest-first by default.
- **`etsy-pp-cli listings find-all-active-by-shop`** - Retrieves a list of all active listings on Etsy in a specific shop, paginated by listing creation date.
- **`etsy-pp-cli listings get`** - Retrieves a listing record by listing ID. Inactive, draft, and expired listings are visible only to the listing's owner, using an access token with the listings_r scope.
- **`etsy-pp-cli listings get-by-ids`** - Allows to query multiple listing ids at once. Limit 100 ids maximum per query. Inactive, draft, and expired listings are returned only to the listing's owner, using an access token with the listings_r scope, and are omitted from the results for everyone else.
- **`etsy-pp-cli listings get-by-shop`** - Endpoint to list Listings that belong to a Shop. Listings can be filtered using the 'state' param.
- **`etsy-pp-cli listings get-by-shop-receipt`** - Gets all listings associated with a receipt.
- **`etsy-pp-cli listings get-by-shop-return-policy`** - Gets all listings associated with a Return Policy.
- **`etsy-pp-cli listings get-by-shop-section-id`** - Retrieves all the listings from the section of a specific shop.
- **`etsy-pp-cli listings get-featured-by-shop`** - Retrieves Listings associated to a Shop that are featured.
- **`etsy-pp-cli listings get-properties`** - Get a listing's properties
- **`etsy-pp-cli listings get-property`** - Development for this endpoint is in progress. It will only return a 501 response.

Retrieves a listing's property
- **`etsy-pp-cli listings get-shipping-by-ids`** - Retrieves the shipping profile for each listing referenced by listing ID. Requires the `shops_r` OAuth scope. Limit 100 listing IDs per request. All requested listing IDs must exist — if any single ID is not found, the entire request returns a 404. Shipping profile data (including `shipping_profile_id`) is only returned for listings owned by the authenticated user; it is nulled out for listings owned by other sellers.
- **`etsy-pp-cli listings update`** - Updates a listing, identified by a listing ID, for a specific shop identified by a shop ID. Note that this is a PATCH method type. When activating, or manually renewing a physical listing, the shipping profile referenced by the `shipping_profile_id`, and all of its fields, along with its entries and upgrades must be complete and valid. If the shipping profile is not complete and valid, we will throw an exception with an error message that guides the request sender to update whatever data is bad. Digital listings that are not made to order must have a file upload associated with it to be activated. While the listing is a draft, shipping profile and file upload are not required in any case.
- **`etsy-pp-cli listings update-property`** - Updates or populates the properties list defining product offerings for a listing. Each offering requires both a `value` and a `value_id` that are valid for a `scale_id` assigned to the listing or that you assign to the listing with this request.

### payments

Manage payments

- **`etsy-pp-cli payments get`** - Retrieves a list of payments from a shop identified by `shop_id`. You can also filter results using a list of payment IDs.
- **`etsy-pp-cli payments get-account-ledger-entry`** - Get a Payment from a PaymentAccount Ledger Entry ID, if applicable
- **`etsy-pp-cli payments get-shop-by-receipt-id`** - Retrieves a payment from a specific receipt, identified by `receipt_id`, from a specific shop, identified by `shop_id`

### processing_profiles

Manage processing profiles

- **`etsy-pp-cli processing-profiles create-shop-readiness-state-definition`** - Creates a new ReadinessStateDefinition. If an existing definition matches the input values, this endpoint will throw a Conflict error, please refer to the Content-Location header to obtain the get endpoint url for the values of the existing definition. Does not affect the product offering-readiness states definition relationship.
- **`etsy-pp-cli processing-profiles delete-shop-readiness-state-definition`** - Deletes a ReadinessStateDefinition by given readiness state definition ID. If there any active offerings linked to the definition, this endpoint will throw a Bad Request error. If you want to delete a ReadinessStateDefinition that is linked to active offerings, you must link the offerings to a different readiness state definition.
- **`etsy-pp-cli processing-profiles get-shop-readiness-state-definition`** - Retrieves a ProcessingProfile referenced by readiness state definition ID.
- **`etsy-pp-cli processing-profiles get-shop-readiness-state-definitions`** - Retrieves a list of ProcessingProfiles available in the specific Etsy shop identified by its shop ID.
- **`etsy-pp-cli processing-profiles update-shop-readiness-state-definition`** - Updates an existing ReadinessStateDefinition. If an existing definition matches the input values, this endpoint will throw a Conflict error, please refer to the Content-Location header to obtain the get endpoint url for the values of the existing definition. Does not affect the product offering-readiness states definition relationship.

### production_partners

Manage production partners

- **`etsy-pp-cli production-partners <shop_id>`** - Retrieves a list of production partners available in the specific Etsy shop identified by its shop ID.

### receipts

Manage receipts

- **`etsy-pp-cli receipts create-shipment`** - Submits tracking information for a Shop Receipt, which creates a Shop Receipt Shipment entry for the given receipt_id. Each time you successfully submit tracking info, Etsy sends a notification email to the buyer User. When send_bcc is true, Etsy sends shipping notifications to the seller as well. When tracking_code and carrier_name aren't sent, the receipt is marked as shipped only. If the carrier is not supported, you may use `other` as the carrier name so you can provide the tracking code. **NOTES** When shipping within the United States AND the order is over $10 _or_ when shipping to India, tracking code and carrier name ARE required. **ACCESS RESTRICTION** In regions where Etsy's Preferred Partner Program is enforced, this endpoint is only available to applications that are approved fulfillment partners. Requests from other applications return `403 Forbidden`, even when the API key is valid, the OAuth token has the `transactions_w` scope and other receipt endpoints respond successfully. A request is authorized if the application is a commercial-access application categorized as Fulfillment or Print On Demand, if it is a personal-use application owned by the authenticated seller, or if the seller authorized the application before the Preferred Partner Program launched in their region. If your application doesn't meet these criteria, contact Etsy API support to ask about partner eligibility. Access to ShopReceipt's first_line, second_line, city, state, zip, country_iso and formatted_address is contingent in some regions to a preferred partnership status with Etsy
- **`etsy-pp-cli receipts get-shop`** - Requests the Shop Receipts from a specific Shop, unfiltered or filtered by receipt id range or offset, date, paid, and/or shipped purchases. **NOTE** Access to ShopReceipt's first_line, second_line, city, state, zip, country_iso and formatted_address is contingent in some regions to a preferred partnership status with Etsy
- **`etsy-pp-cli receipts get-shop-shops`** - Retrieves a receipt, identified by a receipt id, from an Etsy shop. **NOTE** Access to ShopReceipt's first_line, second_line, city, state, zip, country_iso and formatted_address is contingent in some regions to a preferred partnership status with Etsy
- **`etsy-pp-cli receipts update-shop`** - Updates the status of a receipt, identified by a receipt id, from an Etsy shop. **NOTE** Access to ShopReceipt's first_line, second_line, city, state, zip, country_iso and formatted_address is contingent in some regions to a preferred partnership status with Etsy

### return_policies

Manage return policies

- **`etsy-pp-cli return-policies consolidate-shop`** - Consolidates Return Policies by moving all listings from a source return policy to a destination return policy, and deleting the source return policy. This is commonly used in the event that a user attempts to update a Return Policy such that its data is a duplicate of some other Return Policy, which is prevented.
- **`etsy-pp-cli return-policies create-shop-return-policy`** - Creates a new Return Policy. Note: if either accepts_returns or accepts_exchanges is true, then a return_deadline is required.
- **`etsy-pp-cli return-policies delete-shop-return-policy`** - Deletes an existing Return Policy. Deletion is only allowed for policies which have no associated listings – move them to another policy before attempting deletion.
- **`etsy-pp-cli return-policies get-shop`** - Returns a shop's list of existing Return Policies
- **`etsy-pp-cli return-policies get-shop-return-policy`** - Retrieves an existing Return Policy.
- **`etsy-pp-cli return-policies update-shop-return-policy`** - Updates an existing Return Policy. Note: if either accepts_returns or accepts_exchanges is true, then a return_deadline is required.

### reviews

Manage reviews

- **`etsy-pp-cli reviews get-by-listing`** - Open API V3 to retrieve the reviews for a listing given its ID.
- **`etsy-pp-cli reviews get-by-shop`** - Open API V3 to retrieve the reviews from a shop given its ID.

### seller_taxonomy

Manage seller taxonomy

- **`etsy-pp-cli seller-taxonomy get-nodes`** - Retrieves the full hierarchy tree of seller taxonomy nodes.
- **`etsy-pp-cli seller-taxonomy get-properties-by-taxonomy-id`** - Retrieves a list of product properties, with applicable scales and values, supported for a specific seller taxonomy ID.

### shipping_profiles

Manage shipping profiles

- **`etsy-pp-cli shipping-profiles create-shop`** - Creates a new ShippingProfile. You can pass a country iso code or a region when creating a ShippingProfile, but not both. Only one is required. You must pass either a shipping_carrier_id AND mail_class, or both min and max_delivery_days.
- **`etsy-pp-cli shipping-profiles create-shop-destination`** - Creates a new shipping destination, which sets the shipping cost, carrier, and class for a destination in a [shipping profile](/documentation/reference/#tag/Shop-ShippingProfile). createShopShippingProfileDestination assigns costs using the currency of the associated shop. Set the destination using either `destination_country_iso` or `destination_region`; `destination_country_iso` and `destination_region` are mutually exclusive — set one or the other. Setting both triggers error 400. If the request sets neither `destination_country_iso` nor `destination_region`, the default destination is "everywhere". You must also either assign both a `shipping_carrier_id` AND `mail_class` or both `min_delivery_days` AND `max_delivery_days`.
- **`etsy-pp-cli shipping-profiles create-shop-upgrade`** - Creates a new shipping profile upgrade, which can establish a price for a shipping option, such as an alternate carrier or faster delivery.
- **`etsy-pp-cli shipping-profiles delete-shop`** - Deletes a ShippingProfile by given id.
- **`etsy-pp-cli shipping-profiles delete-shop-destination`** - Deletes a shipping destination and removes the destination option from every listing that uses the associated shipping profile. A shipping profile requires at least one shipping destination, so this endpoint cannot delete the final shipping destination for any shipping profile. To delete the final shipping destination from a shipping profile, you must [delete the entire shipping profile](/documentation/reference/#operation/deleteShopShippingProfile).
- **`etsy-pp-cli shipping-profiles delete-shop-upgrade`** - Deletes a shipping profile upgrade and removes the upgrade option from every listing that uses the associated shipping profile.
- **`etsy-pp-cli shipping-profiles get-shipping-carriers`** - Retrieves a list of available shipping carriers and the mail classes associated with them for a given country
- **`etsy-pp-cli shipping-profiles get-shop`** - Retrieves a list of shipping profiles available in the specific Etsy shop identified by its shop ID.
- **`etsy-pp-cli shipping-profiles get-shop-destinations-by`** - Retrieves a list of shipping destination objects associated with a shipping profile.
- **`etsy-pp-cli shipping-profiles get-shop-shops`** - Retrieves a Shipping Profile referenced by shipping profile ID.
- **`etsy-pp-cli shipping-profiles get-shop-upgrades`** - Retrieves the list of shipping profile upgrades assigned to a specific shipping profile.
- **`etsy-pp-cli shipping-profiles update-shop`** - Changes the settings in a shipping profile. You can pass a country iso code or a region when updating a ShippingProfile, but not both. Only one is required. You must pass either a shipping_carrier_id AND mail_class, or both min and max_delivery_days.
- **`etsy-pp-cli shipping-profiles update-shop-destination`** - Updates an existing shipping destination, which can set or reassign the shipping cost, carrier, and class for a destination.
- **`etsy-pp-cli shipping-profiles update-shop-upgrade`** - Updates a shipping profile upgrade and updates any listings that use the shipping profile.

### shop_sections

Manage shop sections

- **`etsy-pp-cli shop-sections create`** - Creates a new section in a specific shop.
- **`etsy-pp-cli shop-sections delete`** - Deletes a section in a specific shop given a valid shop_section_id.
- **`etsy-pp-cli shop-sections get`** - Retrieves the list of shop sections in a specific shop identified by shop ID.
- **`etsy-pp-cli shop-sections get-shops`** - Retrieves a shop section, referenced by section ID and shop ID.
- **`etsy-pp-cli shop-sections update`** - Updates a section in a specific shop given a valid shop_section_id.

### shops

Manage shops

- **`etsy-pp-cli shops find`** - Open API V3 endpoint for searching shops by name. Note: We make every effort to ensure that frozen or removed shops are not included in the search results. However, rarely, due to timing issues, they may appear.
- **`etsy-pp-cli shops get`** - Retrieves the shop identified by a specific shop ID.
- **`etsy-pp-cli shops get-by-owner-user-id`** - Retrieves the shop identified by the shop owner's user ID.
- **`etsy-pp-cli shops update`** - Updates a shop. Assumes that all string parameters are provided in the shop's primary language. Please note that the policy_additional field should only be set for shops located in the EU. Passing a value for this field for shops outside of the EU, will result in an error.

### transactions

Manage transactions

- **`etsy-pp-cli transactions get-shop-receipt`** - Retrieves a transaction by transaction ID.
- **`etsy-pp-cli transactions get-shop-receipt-by-listing`** - Retrieves the list of transactions associated with a listing.
- **`etsy-pp-cli transactions get-shop-receipt-by-receipt`** - Retrieves the list of transactions associated with a specific receipt.
- **`etsy-pp-cli transactions get-shop-receipt-by-shop`** - Retrieves the list of transactions associated with a shop.

### user_addresses

Manage user addresses

- **`etsy-pp-cli user-addresses get`** - Open API V3 endpoint to retrieve UserAddresses for a User.
- **`etsy-pp-cli user-addresses get-user-address`** - Open API V3 endpoint to retrieve a UserAddress for a User.

### users

Manage users

- **`etsy-pp-cli users get`** - Retrieves a user profile based on a unique user ID. Access is limited to profiles of the authenticated user or linked buyers. For the primary_email field, specific app-based permissions are required and granted case-by-case.
- **`etsy-pp-cli users get-me`** - Returns basic info for the user making the request.


### inventory


- **`etsy-pp-cli inventory audit`** - Spot zero-quantity variations, missing or duplicate SKUs and price spread across variations.


### listings


- **`etsy-pp-cli listings audit`** - Find every listing with missing tags, materials, alt text or photos, or missing GPSR manufacturer/responsible-person data, in one offline pass.

- **`etsy-pp-cli listings expiring`** - List active listings expiring soon and whether they auto-renew.

- **`etsy-pp-cli listings dead-weight`** - Find active listings with no sales in a window, with favorite counts.


### market


- **`etsy-pp-cli market snapshot`** - Capture a dated snapshot of the listings and shops that appear for your search terms.

- **`etsy-pp-cli market trend`** - See which listings and shops entered or left, and how prices, favorites and positions moved, between two snapshots.

- **`etsy-pp-cli market demand`** - Rank listings and search terms by recent review volume and shop sales, with an explicit can't-tell result for thin data.

- **`etsy-pp-cli market shops`** - Profile the shops behind a snapshot and see whether big shops crowd out new ones.

- **`etsy-pp-cli market gaps`** - Find targets where no matching listing was found in the pages and terms you searched, with coverage checked against known competitors.

- **`etsy-pp-cli market tags`** - See which tags and title words the best-selling listings use that the rest don't.

- **`etsy-pp-cli market price`** - Price ranges per category and whether price tracks demand.


### money


- **`etsy-pp-cli money fees`** - See exactly what Etsy charged you by fee type for any period.

- **`etsy-pp-cli money reconcile`** - Match receipts, payments and ledger entries by ID and compare receipt totals with summed payment amounts and currencies.


### reviews


- **`etsy-pp-cli reviews digest`** - Rating histogram, low-rated reviews and per-listing averages for a period.


### shop


- **`etsy-pp-cli shop attention`** - One compact summary of what needs attention: unshipped orders, expiring listings, audit findings, reconcile mismatches and sync age.




### Shop management (hand-written)

Generated endpoint commands that send PUT, PATCH, POST or DELETE requests need `--yes`; without it they refuse and exit 2 (`--dry-run` previews them). The commands below are the hand-written shop tools.

- **`etsy-pp-cli listings bulk-edit`** - Edit many listings from a plan file or flags. The default run is a preview with no API calls; nothing is sent until `--apply --yes`. Needs the listings_w scope (`auth login --all-scopes`).
- **`etsy-pp-cli listings copy`** - Build a draft from an existing listing; previews it with `--dry-run` and creates it only with `--yes`.
- **`etsy-pp-cli listings snapshot`** - Save a listing, its inventory and personalization to the local store, or `--list` the saved history.
- **`etsy-pp-cli inventory set-qty`** - Change one offering's quantity; shows the old and new value and writes only with `--yes`.
- **`etsy-pp-cli inventory low-stock`** - List offerings at or below `--threshold`, from the local store.
- **`etsy-pp-cli orders unshipped`** - Paid, unshipped receipts from the local store, oldest first; PII-minimized unless `--full`.
- **`etsy-pp-cli money summary`** - Gross, fees, refunds, payouts and net from the synced ledger over a window.

### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`etsy-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`etsy-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`etsy-pp-cli learnings list`** - Inspect taught rows
- **`etsy-pp-cli learnings forget <query>`** - Undo a teach
- **`etsy-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`etsy-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`etsy-pp-cli teach-pattern`** - Install a query/resource template up front
- **`etsy-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ETSY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `etsy-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
etsy-pp-cli listings get 1000000001

# JSON for scripting and agents, filtered to specific fields
etsy-pp-cli listings get 1000000001 --json --select listing_id,title,state

# Dry run - show the request without sending
etsy-pp-cli listings get 1000000001 --json --select listing_id,title,state --dry-run

# Agent mode - JSON + compact + no prompts in one flag
etsy-pp-cli listings get 1000000001 --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `ETSY_SHOP_ID` resolves `{shop_id}`

Base URL: `https://openapi.etsy.com/v3/application`

## Health Check

```bash
etsy-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `etsy-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/etsy-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\etsy-pp-cli\config.toml` on Windows; `--home`, `ETSY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `ETSY_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `ETSY_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ETSY_API_KEY` | per_call | Yes | Etsy app keystring (or `keystring:shared_secret`). |
| `ETSY_SHARED_SECRET` | per_call | Yes, unless `ETSY_API_KEY` is `keystring:secret` | Etsy app shared secret. Sensitive. |
| `ETSY_SHOP_ID` | endpoint | For shop data | Numeric shop id; resolves `{shop_id}`. |
| `ETSY_OAUTH2` | per_call | No | Pasted OAuth bearer token; `auth login` normally supplies it. |
| `ETSY_REDIRECT_URI`, `ETSY_SCOPES`, `ETSY_CLIENT_ID` | auth login | No | Override the `auth login` redirect URI, scopes and OAuth client id. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `etsy-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `etsy-pp-cli doctor` to check credentials
- Check that `ETSY_API_KEY` (keystring) and `ETSY_SHARED_SECRET` are set, without printing them: `[ -n "$ETSY_API_KEY" ] && [ -n "$ETSY_SHARED_SECRET" ] && echo set`
- Shop data (orders, money, drafts, inventory) also needs an OAuth login: `etsy-pp-cli auth login`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Shop-scoped commands need `ETSY_SHOP_ID` set to your shop id

### API-specific
- **403 "API key not found or not active, or incorrect shared secret"** — Set ETSY_SHARED_SECRET (or ETSY_API_KEY=keystring:secret) and confirm the app is active at etsy.com/developers/your-apps; then run `etsy-pp-cli doctor`.
- **429 Too Many Requests** — Wait for the retry-after seconds the CLI prints; use local commands after `sync` instead of repeated live calls.
- **403 on orders, ledger or write commands** — Check granted scopes with `etsy-pp-cli app token-scopes`; for writes, log in again with `etsy-pp-cli auth login --all-scopes`.
- **Reports show stale data** — Run `etsy-pp-cli sync --since 7d` to refresh the local store.
- **Shop commands return nothing after sync** — Set ETSY_SHOP_ID to your shop id and run `etsy-pp-cli auth login`; sync skips shop data with a hint when either is missing.

## Known Gaps

- **Listing prune needs a complete, unnarrowed `sync --full` with OAuth.** Local listings Etsy no longer returns are removed only after a full walk of all five states (active, inactive, draft, expired, sold_out). Without an OAuth login, with `--resources` narrowing, `--max-pages`, `--latest-only` or a failed state walk, nothing is pruned.
- **An explicit `--resources listings_inventory` walk does not replace ghost rows.** The default sync (which embeds inventory in the listings walk) replaces each listing's inventory rows; the per-listing dependent walk only adds and updates.
- **First sync after upgrading re-walks payments once.** Stores from before the check ledger carry no per-receipt checks, so the first sync after upgrading checks every receipt's payments once (resumable: capped runs continue where they stopped).
- **404 receipts are skipped; 403 is probed once per run.** When Etsy answers a receipt's `/payments` with 404, the receipt is recorded as `not_found` and skipped on later runs until its `updated_timestamp` changes (or you pass `--full`); the sync summary reports how many. A 404 does not hold the payments attempt incomplete. A 403 (which may be specific to an inaccessible receipt) is recorded as `forbidden`: every later run checks unchecked, re-armed and changed receipts first, then makes exactly ONE probe request for the forbidden receipt that was checked longest ago (also under `--full`, where forbidden receipts sort after every other receipt). The quota cost of a standing 403 is therefore one `/payments` call per run. If the probe succeeds, every forbidden row is re-armed in the same transaction and those receipts are checked from the next run; if it still returns 403, the rows stay. The payments attempt stays `partial` while any forbidden row exists, and each run emits a `payments_forbidden` warning with the current ledger count. Receipt payment 403s retry next sync. A run with both ok checks and 403s warns and clears `ever_ok` on its forbidden receipts. With no ok checks, a 403 on an `ever_ok` receipt triggers exactly one verification request on a different, unwalked `ever_ok` receipt, within the call and quota budget. A successful verification records ok and clears `ever_ok` on this run's forbidden receipts; a forbidden or unavailable verification preserves history and exits 1 with the `transactions_r` scope hint. No prior successful check also exits 1. `--strict` keeps its strict reason and includes the scope hint when applicable. Capped runs exit 7, except zero-progress runs with a scope problem, which exit 1. 5xx and transport errors record nothing and are retried every run. Ledger rows written by older builds as `denied` (403 or 404, origin unknown) are re-armed once on the first write after upgrading, so those receipts are re-checked once.
- **MCP writes.** The MCP write path for endpoints is `etsy_execute` with `confirm:true`; hand-written write commands are preview-only over MCP.
- **Capped listings syncs restart.** A `sync --max-pages N` run over listings does not resume from where it stopped; the next run starts each state from the first page.
- **Shop features are verified offline only so far.** Orders, money, inventory, and non-active listings need an OAuth login to a real shop. Their commands are covered by tests against synthetic data; live verification against a shop is still pending.
- **Shop counts can be unknown.** When a resource was never synced, `shop attention` reports `null` (not `0`) plus a note saying the count is unknown; read the `notes` field.
- **Union-prune state-change window (deferred).** The listings prune compares stored rows with the ids seen across the five state walks. A listing that changes state while the walk is running can be missed or pruned until the next sync repairs the cache.
- **Embedded `includes=Images,Inventory` is live-unverified.** The listings walk asks for embedded images and inventory (confirmed in the Etsy spec, not against a real shop); live verification is a Phase 18 item.
- **Live per-endpoint MCP tools return raw API responses.** When the operator's OAuth token has `transactions_r`, these responses include buyer data. This is a generator-level gap in the generic `makeAPIHandler`, not specific to one command. The MCP conformance test skips the tools that only talk to the live API (`etsy_read`, `etsy_execute`, `sync`, `export` (fetches from the live API), `market_snapshot`, `listings_copy`, `inventory_set_qty`, `import`); the live per-endpoint tools are not covered by the buyer guard.
- **Over MCP, `recall` still returns a buyer hit's `resource_type` and `resource_id`.** The entities are dropped, but the type and id of a buyer-typed hit remain visible. `recall` also returns a buyer learning's `resource_id` and `resource_type` (never names) for a matching query, plus a matched playbook's notes; only its candidate section is filtered.
- **Buyer learnings are hidden over MCP by token overlap.** A candidate or playbook family is matched against buyer learnings' query patterns by shared tokens. The match over-hides (an unrelated family sharing a token such as `orders` with a buyer learning is hidden), and it does not catch buyer text that is not linked to a buyer-typed learning (for example a playbook note written by hand). It fails closed only when the learnings index cannot load.
- **A 403 from `/payments` alone may not be a scope problem.** Receipt payment 403s retry next sync. A run with both ok checks and 403s warns and clears `ever_ok` on its forbidden receipts. With no ok checks, a 403 on an `ever_ok` receipt triggers exactly one verification request on a different, unwalked `ever_ok` receipt, within the call and quota budget. A successful verification records ok and clears `ever_ok` on this run's forbidden receipts; a forbidden or unavailable verification preserves history and exits 1 with the `transactions_r` scope hint. No prior successful check also exits 1. `--strict` keeps its strict reason and includes the scope hint when applicable. Capped runs exit 7, except zero-progress runs with a scope problem, which exit 1.
- **The MCP-start lookup purge runs only from `cmd/etsy-pp-mcp/main.go`.** Legacy buyer-kind lookups are purged once when that server starts; other entry points that build the MCP server do not run it. The call is covered by `TestEtsyMCPStartPurgeRunsAfterFlagParse`.
- **The conformance opt-in arm shows 9 tools fire in read-only and 11 in read-write: `sql`, `search`, `recall`, `orders_unshipped`, `reviews_digest`, `analytics`, `learnings_list`, `learnings_candidates`, `playbook_list`, plus `learnings_confirm` and `learnings_reject` in read-write.** The rest are asserted non-buyer-reading by a reviewed map (`etsyNonBuyerReading`); each entry names what the tool reads and was checked against source in round 9. A new tool that starts reading buyer rows fails the arm only if it emits a sentinel.
- **`teach` and `teach-pattern` refusals are silent over MCP (`SilenceErrors`).** Conformance attributes them to the guard by an opt-in control run, not by message text.
- **MCP search is filtered by stored resource type.** Any unclassified type is hidden, including custom types the operator syncs.
- **Over MCP, `economic_operators` is treated as buyer-bearing.** This is conservative: the type is hidden unless buyer-PII access is enabled.
- **`learn_recall_misses` is stored unminimised.** It keeps the agent/operator's recall query text, which may include buyer names typed into recall. Over MCP `sql` denies the table (like the other learn tables) unless the buyer-PII opt-in is on; minimising it at write time is deferred.
- **Low-stock `--refresh` split heuristic.** When a batch inventory read fails, both halves are re-read and split again, down to single listings, until the per-chunk split budget (`len(chunk)+8` calls) runs out; listings still unread at that point are counted as failed. A pathological mix can leave some good listings counted as failed. It surfaces as the "N of M inventory reads failed" warning.
- **The MCP search envelope always carries `buyer_pii: "filtered"` when the opt-in is off.** Before, it appeared only when a buyer row was dropped.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**etsy-mcp-server (administrativetrick)**](https://github.com/administrativetrick/etsy-mcp-server) — JavaScript (21 stars)
- [**etsy-mcp (aserper)**](https://github.com/aserper/etsy-mcp) — TypeScript (13 stars)
- [**etsy-seller-mcp-hub**](https://github.com/Bl0ck154/etsy-seller-mcp-hub) — JavaScript (3 stars)
- [**etsy-mcp (mofiaboss)**](https://github.com/mofiaboss/etsy-mcp) — Python (2 stars)
- [**etsy-v3-api-client + etsy-cli**](https://github.com/profplum700/etsy-v3-api-client) — TypeScript (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

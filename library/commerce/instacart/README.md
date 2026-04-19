# instacart

Agent-native command line client for Instacart. Manage your cart, list active
carts across retailers, and add items by natural language or item id - all
through direct GraphQL replay against Instacart's web API, using the session
you already have in Chrome.

**No browser automation. No Playwright. No Composio subscription. Just a binary.**

## Why this exists

Instacart's official API is for affiliate landing pages (recipes, shopping
list pages), not real shopper operations. Every other tool that does manage a
real cart uses full browser automation - spawning Playwright per call, 20-40
seconds per action, constant bot-detection fights. This CLI talks directly to
Instacart's GraphQL endpoint with your session cookies, so a cart add takes
under a second.

The killer workflow: tell your agent "add 2% milk to my Costco cart" and the
item is waiting for you next time you open the app.

## Quick Start

```bash
# 1. Build (requires Go 1.23+)
go build -o instacart ./cmd/instacart

# 2. Seed the persisted-query hash cache
./instacart capture

# 3. Log in (reads cookies from Chrome via kooky)
./instacart auth login

# If kooky fails to decrypt (recent Chrome on macOS has stricter Keychain
# protection), fall back to the file-based import:
./instacart auth import-file /path/to/cookies.json

# Or paste a Cookie header from devtools:
./instacart auth paste

# 4. Verify
./instacart doctor
```

## First-time history backfill

Optional but recommended. Once backfilled, `add` resolves items from your real purchase history ("Alden's Organic Limoncello Sorbet Bars" instead of whatever live search ranks first for "limoncello sorbet") and runs about 5x faster on repeated items.

The fastest path is the pp-instacart skill. In a Claude Code session with `claude-in-chrome` MCP tools loaded, tell the agent:

> backfill my instacart orders

The skill walks your logged-in Chrome tab through every order, extracts the data, and runs `instacart history import` for you. Typical full backfill: 5-10 minutes for a ~180-order history. Subsequent top-ups: under a minute.

If you do not have `claude-in-chrome` MCP available, run the flow manually: see [`docs/backfill-devtools-fallback.md`](docs/backfill-devtools-fallback.md). Same three JS files, same import command, different driver.

More detail: [`docs/backfill-walkthrough.md`](docs/backfill-walkthrough.md) (full procedure) and [`docs/patterns/authenticated-session-scraping.md`](../../../docs/patterns/authenticated-session-scraping.md) (why this pattern exists).

## Agent Usage

Every command supports `--json` for structured output and typed exit codes
for composability:

```bash
# Exit code 0 on success, 3 auth, 4 not-found, 5 conflict, 7 transient
instacart carts --json | jq '.[] | {name: .retailer.name, items: .itemCount}'

# Add a known product to a cart, non-interactively, with JSON output
instacart add --item-id items_1576-17315429 costco --qty 1 --yes --json

# Dry-run to preview without firing the mutation
instacart add --item-id items_1576-17315429 costco --dry-run --json
```

## Commands

### The killer one

```
instacart add <retailer-slug> <query...> [--qty N] [--yes] [--dry-run]
instacart add --item-id <items_LOC-PROD> <retailer-slug> [--qty N] [--yes]
```

Resolves a product from a natural-language query and fires
`UpdateCartItemsMutation` against your live cart. Three direct GraphQL
round trips under the hood (ShopCollectionScoped -> Autosuggestions ->
Items), all under a second. No browser, no Playwright, no MCP subscription.

- `--dry-run` previews without firing the mutation
- `--yes` skips the confirmation prompt
- `--qty` sets quantity (default 1)
- `--cart-id` lets you target an explicit cart (otherwise resolved from your active carts)
- `--item-id` is an override for power users or agents that already know the exact item id

```
instacart add costco "2% milk"
instacart add pcc-community-markets "organic eggs" --qty 2 --yes
instacart add costco milk --dry-run --json
```

Arg shape: first positional arg is the retailer slug, remaining args join
into the query. The old "query ... retailer" order is still detected when
the last arg matches a known retailer slug, with a one-time deprecation
notice to stderr.

### Cart management

```
instacart carts                               # list every active cart across retailers
instacart cart show <retailer-slug>           # show a specific cart's item count
instacart cart remove <item-id> <retailer>    # remove an item from a cart
```

### Account + discovery

```
instacart retailers list                      # cached retailers (populated from carts + searches)
instacart retailers show <slug>               # look up a retailer's shop id etc
instacart search "<query>" --store <slug>     # product search (best-effort, see note above)
```

### Auth

```
instacart auth login                          # extract Chrome cookies via kooky
instacart auth import-file <path>             # fallback when kooky can't decrypt
instacart auth paste                          # fallback: paste a Cookie header
instacart auth status                         # show current session
instacart auth logout                         # delete saved session
```

### Infrastructure

```
instacart doctor                               # full health check + live API ping
instacart capture                              # re-seed persisted query hashes
instacart ops list                             # show cached GraphQL operation hashes (hidden)
```

## Health Check

`instacart doctor` runs five checks and reports each:

- `config`  - config file at `~/Library/Application Support/instacart/config.json`
- `store`   - SQLite cache at the same directory
- `ops`     - how many persisted GraphQL operation hashes are cached
- `session` - whether an Instacart session is loaded
- `api`     - live `CurrentUserFields` query (exercises the whole stack)

Exit codes: 0 if all pass, 3 on session failure, 7 on API failure.

## Troubleshooting

### `PersistedQueryNotSupported` on mutations

This means Instacart has rolled a new web bundle and the `UpdateCartItemsMutation`
hash baked into this binary is stale. Fix: re-run `instacart capture` for now
(static reseed), or in a future release, `instacart capture --live` to extract
the fresh hash from a headed browser. As a temporary workaround, pin to the
binary version that was current when your session cookies were captured.

### `kooky` returns garbage cookie values

Newer Chrome versions on macOS (v130+) encrypt cookies with a Keychain-stored
key that `kooky` can't always decrypt. Symptoms: `auth login` says "imported N
cookies" but `doctor` reports `auth rejected (HTTP 401)`. Workarounds:

1. `instacart auth import-file <path>` using a JSON export from a Playwright
   session or another tool that reads cookies via CDP.
2. `instacart auth paste` and paste the Cookie header value straight from
   Chrome devtools.

### "not logged in" after running `auth login`

Chrome has to be logged in to `https://www.instacart.com` at the moment you
run `auth login`. If you're logged out in Chrome, the session cookie isn't
there to extract.

### "no active cart at <retailer>"

Instacart creates one cart per retailer per customer. `instacart carts` shows
them all. If the retailer you named isn't in that list, you have no active
cart there yet - use `instacart add --item-id <id> <retailer>` to create one
by adding something.

## Cookbook

```bash
# What do I have in all my carts right now?
instacart carts

# Add a known Costco item (pulled from a previous add or the web UI's
# network tab) to my Costco cart, non-interactively
instacart add --item-id items_1576-17315429 costco --qty 1 --yes

# Preview without firing the mutation
instacart add --item-id items_1576-17315429 costco --qty 2 --dry-run --json

# Remove it
instacart cart remove items_1576-17315429 costco

# Check a cart's state via JSON for scripts
instacart cart show costco --json | jq .items
```

## How this was built

Instacart's web client is a React SPA backed by Apollo Client 3 using
persisted queries. Every operation is keyed by an sha256 hash of the query
document, and the server only executes operations whose hash is in its
allowlist. Queries are sent via GET with the hash in `?extensions=...`;
mutations via POST with the same envelope in the body. There is no public API
for shopper cart operations.

This CLI was built by:

1. Sniffing a live browser-use session while walking the add-to-cart flow
2. Extracting the `UpdateCartItemsMutation` hash from Apollo's persistedQueryLink
   by invoking it directly with a mock forward function that captures the
   hash set on `operation.extensions.persistedQuery.sha256Hash`
3. Extracting all GET-query hashes directly from the Performance API resource
   URLs (64 distinct operations across the storefront, search, and cart flows)
4. Reading Instacart session cookies from Chrome's cookie store via the
   `kooky` Go library (with a JSON-file fallback for when kooky fails to
   decrypt on newer Chrome macOS builds)
5. Replaying those hashed operations against `https://www.instacart.com/graphql`
   with the session cookies attached

The local SQLite store keeps the hashes, retailer cache, and cart snapshots
available for offline reads.

## Known Gaps

- **Past order history** - Instacart's `/store/account/orders` page is a live
  tracker, not a history view. Past orders are in the mobile app surface and
  a separate query we haven't captured.
- **Delivery windows** are visible in cart responses but not yet surfaced as
  a standalone command.
- **Cross-shop cart items** (when a cart contains items whose location
  prefix doesn't match the retailer's current default shop) return server
  errors on the `price` field but names still come through. `cart show`
  tolerates this and displays names; price resolution is TBD.
- **The mutation and query hashes are frozen at build time**. When Instacart
  rolls a new web bundle, `instacart capture --remote` will try to fetch an
  updated hash registry from GitHub. That registry is maintained by whoever
  re-sniffs with printing-press after a rotation. If the registry is stale,
  fall back to installing browser-use and re-sniffing locally.
- **Natural-language product search** uses the top autosuggest match. For
  ambiguous queries ("fresh strawberries" vs "frozen strawberries") the
  top result may not match your intent. Use `instacart search` first to see
  the ranked list, then pass `--item-id` to `add` for precision.

## Install

Built binary lives at `~/printing-press/library/instacart/instacart`. Symlink
it into your PATH:

```bash
ln -s ~/printing-press/library/instacart/instacart ~/bin/instacart
```

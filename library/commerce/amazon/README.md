# amazon-pp-cli

Agent-native command line client for **buyer-side** Amazon.com — reorder previously-purchased items from the cart, with full checkout, from a single binary.

Talks directly to amazon.com using your Chrome session cookies. Sub-second add-to-cart, no Playwright running at runtime. Repurchase-only by design — the LLM can't invent a new SKU. Multi-account via `--profile`.

## Why this exists

Every other Amazon CLI / MCP server either:

- wraps the Product Advertising API (affiliate-only, no cart, no checkout, no order history), or
- spawns Playwright per call (slow, brittle, needs a browser running)

This one replays plain HTTP against amazon.com with your existing session cookies. The trade-off: a one-time history dump from your browser (`docs/dumper.js`) seeds the SQLite store that powers history-first add resolution.

## Quick Start

```bash
# 1. Build
go build -o amazon-pp-cli ./cmd/amazon-pp-cli

# 2. Create profiles for each Amazon account you use
./amazon-pp-cli profiles add personal --label "Personal account"
./amazon-pp-cli profiles add work --label "Work account"

# 3. Import session cookies for one of them
#    (kooky reads Chrome's cookie store directly — when Keychain blocks it,
#    fall back to `auth paste` and paste a Cookie header from DevTools)
./amazon-pp-cli --profile personal auth login
./amazon-pp-cli --profile personal doctor

# 4. Backfill order history (one-time per profile)
#    Paste docs/dumper.js into the DevTools console on amazon.com/your-orders
#    Save the output to orders.jsonl, then:
./amazon-pp-cli --profile personal history import ./orders.jsonl

# 5. Use it
./amazon-pp-cli --profile personal history search 'bath tissue'
./amazon-pp-cli --profile personal add 'bath tissue' --dry-run
./amazon-pp-cli --profile personal add 'bath tissue'
./amazon-pp-cli --profile personal cart show
./amazon-pp-cli --profile personal checkout --dry-run
./amazon-pp-cli --profile personal checkout --yes
```

## Agent Usage

Every command supports `--json` for machine output. The exit code surface is typed:

| Code | Meaning |
|------|---------|
| 0    | OK |
| 2    | Usage error |
| 3    | Auth error (no session, expired session) |
| 4    | Not found (no profile, no match in history) |
| 5    | Conflict (duplicate profile, save failure) |
| 7    | Transient (amazon.com 5xx, network) |
| 10   | Confirmation required (`checkout` without `--yes`) |

A nanoclaw or other agent should call `amazon-pp-cli add '<query>' --dry-run --json` first to preview the match, surface it to the user, then run the same command without `--dry-run` to commit.

## Unique Features

- **Repurchase-only.** `add` refuses queries that don't match local history. No "first search result" fallback — the safety rail that prevents an agent from buying a random SKU.
- **Most-frequent tiebreak.** When the local history has multiple matches, the most-frequently-purchased one wins (not the most recent). Deterministic and explainable.
- **Per-profile DB isolation.** Each account has its own SQLite file under `~/.config/amazon-pp-cli/profiles/<name>/history.db`. Personal history can't leak into work searches.
- **`reorder-last`.** Re-add every line item from your most recent order in one call.
- **`add --dry-run`.** Preview which past purchase a query would resolve to without writing to the cart.

## Health Check

```bash
./amazon-pp-cli doctor             # full check (auth + DB + amazon.com reachability)
./amazon-pp-cli doctor --skip-live # offline subset
./amazon-pp-cli doctor --json
```

`doctor` reports the profile, whether cookies loaded, whether the session has the at-main + session-id marker pair, history row counts, last purchase timestamp, and the result of a homepage GET against amazon.com (detected account greeting on success).

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `auth login`: keychain access denied | Use `amazon-pp-cli auth paste` — paste the Cookie header from DevTools. Always works. |
| `add`: "no match in history" | Run `history search '<query>'` to see what's loaded. If nothing matches, run `history import` against a fresh JSONL dump first. |
| `cart show`: 0 items even after `add` | Amazon's cart HTML occasionally hides quantity attributes; the line is there, just unparseable. Open `amazon.com/cart` in your browser to confirm. |
| `checkout`: "not confirmed" / "place-order form" | Your cart contains an item that needs a per-item shipping decision (split shipment, gift wrap). Resolve on amazon.com, then re-run. |
| `doctor` reports robot check | Open amazon.com in your browser and clear the challenge, then retry. This CLI never solves CAPTCHAs. |

## Cookbook

```bash
# Restock the household basket
amazon-pp-cli --profile personal reorder-last --dry-run
amazon-pp-cli --profile personal reorder-last
amazon-pp-cli --profile personal checkout --yes

# Audit what 'add X' would resolve to (agent-friendly preview)
amazon-pp-cli --profile personal add 'detergent' --dry-run --json

# Check both accounts for the same item
for p in personal work; do
  amazon-pp-cli --profile "$p" history search 'sticky notes' --json
done

# Switch the default profile
amazon-pp-cli profiles use work
amazon-pp-cli profiles list
```

## Out of scope

By design:

- **New-item search / discovery.** Defeats the repurchase-only safety rail.
- **Subscribe & Save management.** Use amazon.com directly.
- **Prime Pantry / Fresh / Whole Foods.** Different surfaces, different carts.
- **Returns / cancellation.** Mutating an existing order is high-stakes; the CLI refuses by omission.
- **Digital downloads / Kindle.** Different fulfillment.

## Data on disk

```
~/.config/amazon-pp-cli/
  config.json                                  global profile list + active
  profiles/<name>/
    cookies.json                               session cookies (0600 perms)
    history.db                                 SQLite per-profile history
```

## License

MIT. This is a personal-use CLI; respect amazon.com's terms of service.

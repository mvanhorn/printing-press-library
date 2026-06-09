---
name: amazon-pp-cli
description: |
  History-first Amazon reorders from the command line — refuses to buy anything
  the user hasn't bought before. Talks directly to amazon.com via Chrome session
  cookies. Sub-second add-to-cart, no Playwright, multi-account via --profile,
  full checkout gated on explicit --yes. Use for: "reorder X from Amazon",
  "add bath tissue to my Amazon cart", "restock my last Amazon order".
  Do NOT use for: new-item discovery, returns, digital downloads.
trigger_phrases:
  - "reorder X from Amazon"
  - "add X to my Amazon cart"
  - "restock my last Amazon order"
  - "buy that thing I always get from Amazon"
  - "use amazon-pp-cli"
  - "run amazon-pp-cli"
anti_triggers:
  - "search Amazon for X"          # new-item discovery is intentionally not built
  - "return X to Amazon"           # mutating existing orders is out of scope
  - "check Amazon prices"          # no catalog browsing; this CLI is history-only
binary: amazon-pp-cli
---

# amazon-pp-cli

Agent-native Amazon CLI for **history-first repurchases**. The killer property:
`add 'X'` refuses to write to the cart unless the user has bought X before.

## Prerequisites: Install the CLI

This skill drives the `amazon-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install amazon --cli-only
   ```
2. Verify: `amazon-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon/cmd/amazon-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to use

Reach for this when the user wants to restock something they've ordered before.
Examples:

- "add bath tissue to my Amazon cart" → `amazon-pp-cli add 'bath tissue' --dry-run --json`, surface the matched ASIN to the user, then re-run without `--dry-run` once they confirm.
- "reorder my last Amazon order" → `amazon-pp-cli reorder-last --dry-run --json`, show the planned items, then `amazon-pp-cli reorder-last` once confirmed.
- "place the Amazon order" → `amazon-pp-cli checkout --dry-run` to confirm reachability, then `amazon-pp-cli checkout --yes` to actually place it.

Do NOT use this for new-item discovery, returns, cancellations, or digital
content. The CLI deliberately refuses those.

## Multi-account

the user has two profiles: `personal` and `work`. Always pass
`--profile <name>` explicitly when the user's intent is account-specific
("add it to my work cart" → `--profile work`). When the user is
ambiguous, ask which profile.

## Recipes

### Preview and commit a repurchase
```bash
amazon-pp-cli --profile personal add 'bath tissue' --dry-run --json
# inspect matched ASIN, title, purchase_count, last_purchased_at
amazon-pp-cli --profile personal add 'bath tissue'
```

### Reorder the entire last order
```bash
amazon-pp-cli --profile personal reorder-last --dry-run --json
amazon-pp-cli --profile personal reorder-last
amazon-pp-cli --profile personal checkout --yes
```

### Audit history before deciding
```bash
amazon-pp-cli --profile personal history search 'detergent' --json
amazon-pp-cli --profile personal history list --json | head -10
```

### Cart inspection
```bash
amazon-pp-cli --profile personal cart show --json
```

### Health check
```bash
amazon-pp-cli --profile personal doctor --json
# Reports: cookies_loaded, has_marker, history_orders, history_items,
# last_purchased, amazon_reached, detected_account
```

## Exit codes

| Code | Meaning | Agent action |
|------|---------|---------------|
| 0 | OK | continue |
| 2 | usage error | surface flag/arg error to user |
| 3 | auth error | prompt user to run `auth login` or `auth paste` |
| 4 | not found | surface "no history match" — do NOT fall back to a search |
| 5 | conflict | duplicate profile, etc. — surface to user |
| 7 | transient | retry once, then surface |
| 10 | confirmation required | re-run with `--yes` ONLY after explicit user approval |

## JSON shapes

`add --json` returns:
```json
{
  "query": "bath tissue",
  "matched": true,
  "asin": "B07AAA0001",
  "title": "Charmin Ultra Strong 24 Mega Rolls",
  "purchase_count": 2,
  "last_purchased_at": "2026-04-02T00:00:00Z",
  "added": false,
  "dry_run": true,
  "quantity": 1
}
```

`reorder-last --json` returns:
```json
{
  "order_id": "112-0000003-0000003",
  "placed_at": "2026-04-28T00:00:00Z",
  "dry_run": true,
  "items": [
    {"asin": "B07CCC0003", "title": "Tide Pods Original 96-count", "quantity": 1, "added": false}
  ],
  "added_count": 0
}
```

`doctor --json` returns:
```json
{
  "profile": "personal",
  "cookies_loaded": true,
  "has_marker": true,
  "history_orders": 12,
  "history_items": 47,
  "last_purchased": "2026-04-28T00:00:00Z",
  "amazon_reached": true,
  "detected_account": "the user"
}
```

## Safety contract

1. **Repurchase-only.** `add` returns exit 4 on no-history-match. Do not attempt to "interpret" that — surface it to the user as "no past purchase matches 'X'; want me to search Amazon? (the CLI doesn't, but I could open a browser)".
2. **Confirmation gate on checkout.** `checkout` returns exit 10 without `--yes`. Never pass `--yes` until the user has explicitly confirmed the cart contents in the current turn.
3. **--dry-run is safe.** Every mutating command supports `--dry-run` and will not touch amazon.com when it's set. Prefer dry-run-first for any new query.

## Troubleshooting

If `doctor` reports `cookies_loaded: false`:
- `amazon-pp-cli --profile <name> auth login` (kooky), or
- if Keychain denies kooky: `amazon-pp-cli --profile <name> auth paste` and ask the user for a Cookie header copied from `amazon.com` DevTools.

If `add` returns exit 4 unexpectedly:
- `amazon-pp-cli history search '<query>'` to see what the FTS index actually has.
- If the result set is empty, `history import <jsonl>` against a fresh dump from `docs/dumper.js`.

If `cart show` is empty after `add` reported success:
- Amazon's cart HTML occasionally drops the data attributes the parser reads. The add likely succeeded server-side. Have the user open amazon.com/cart in their browser to confirm.

If `doctor` reports a robot check:
- The CLI never solves CAPTCHAs. Have the user open amazon.com, clear the challenge, then retry.

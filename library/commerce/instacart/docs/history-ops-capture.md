# Capturing Instacart history hashes — SUPERSEDED

**This document is superseded.**

The original PR #78 assumed Instacart exposed dedicated GraphQL ops named `BuyItAgainPage` and `CustomerOrderHistory`. It does not. The `history sync` command that relied on those ops cannot be made to work because the underlying operations are fictional — see [`docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md`](../../../docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md).

## Use this instead

For the working end-user flow:

- **One command:** Tell the `/pp-instacart` skill "backfill my instacart orders". It drives the full Chrome MCP + import loop for you.
- **Walkthrough:** [`backfill-walkthrough.md`](backfill-walkthrough.md) covers the skill-driven path end to end.
- **Manual fallback:** [`backfill-devtools-fallback.md`](backfill-devtools-fallback.md) for users without `claude-in-chrome` MCP.

Technical reference:

- **Playbook:** [`docs/patterns/authenticated-session-scraping.md`](../../../docs/patterns/authenticated-session-scraping.md) — tiered decision tree for this class of problem across any printed CLI.
- **Scripts:** `docs/dumper.js`, `docs/extract-one.js`, `docs/export-jsonl.js` — the browser-side JS.
- **Importer:** `instacart history import <path>` — the CLI-side command.

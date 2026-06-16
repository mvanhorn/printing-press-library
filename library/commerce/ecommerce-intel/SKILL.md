# Ecommerce Intel Skill

Use `ecommerce-intel-pp-cli` when an agent needs private ecommerce intelligence for Shopify-style storefronts.

## Safe defaults

- Prefer `--agent` for JSON, compact, no-input output.
- Run `sync` with no flags for the embedded local fixture.
- Use `sync --import <dataset.json>` for local client exports.
- `--source`, `--live`, and `--real` are adapter-plan modes in this MVP and do not require child CLIs.
- Never print secrets. `doctor` and `sources doctor` report env var presence only.

## High-value workflows

```bash
ecommerce-intel-pp-cli --agent agent-context
ecommerce-intel-pp-cli --agent doctor
ecommerce-intel-pp-cli --profile store sync
ecommerce-intel-pp-cli --profile store dashboard
ecommerce-intel-pp-cli --profile store opportunities --limit 5
ecommerce-intel-pp-cli --profile store action-plan
ecommerce-intel-pp-cli --profile store geo-audit
```

For Shopify marketing pages and collections, start with:

1. `money-pages`
2. `money-products`
3. `category-actions`
4. `product-actions`
5. `geo-audit`

For revenue drops, run:

```bash
ecommerce-intel-pp-cli --profile store explain-drop <product-or-query>
ecommerce-intel-pp-cli --profile store query-revenue <topic>
ecommerce-intel-pp-cli --profile store inventory-risk
```

## Source adapters

The source plan covers Shopify, Klaviyo, GA4, GSC, and Ahrefs child CLIs plus local JSON fixtures/import. Use `sources doctor` to confirm which optional binaries and env vars are available before attempting real integration work.

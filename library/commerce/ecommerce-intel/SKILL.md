# Ecommerce Intel Skill

Use `ecommerce-intel-pp-cli` when an agent needs private ecommerce intelligence for Shopify-style storefronts.

## Safe defaults

- Prefer `--agent` for JSON, compact, no-input output.
- Run `sync` with no flags for the embedded local fixture.
- Use `sync --import <dataset.json>` for local client exports.
- Use `sync --source <source>` for explicit child CLI live sync. `--live` and `--real` alias `--source all`; all-source sync requires Shopify, Klaviyo, GA4, GSC, and Ahrefs configuration before any child runs.
- Single-source sync merges into the existing local dataset and preserves unmatched products/pages as orphan evidence rather than replacing the store model.
- Run `confidence` before trusting forecast output.
- Run `movers` after at least two syncs to find commerce climbers, droppers, new Strike Zone entrants, and new revenue-at-risk surfaces.
- Never print secrets. `doctor` and `sources doctor` report env var presence only.

## High-value workflows

```bash
ecommerce-intel-pp-cli --agent agent-context
ecommerce-intel-pp-cli --agent doctor
ecommerce-intel-pp-cli sync --profile store
ecommerce-intel-pp-cli movers --profile store
ecommerce-intel-pp-cli confidence --profile store
ecommerce-intel-pp-cli dashboard --profile store
ecommerce-intel-pp-cli opportunities --profile store --limit 5
ecommerce-intel-pp-cli action-plan --profile store
ecommerce-intel-pp-cli geo-audit --profile store
ecommerce-intel-pp-cli source-coverage --profile store --missing-only
ecommerce-intel-pp-cli forecast-impact --profile store
```

For Shopify marketing pages and collections, start with:

1. `money-pages`
2. `money-products`
3. `category-actions`
4. `product-actions`
5. `merchandising-link-plan`
6. `geo-audit`

For revenue drops, run:

```bash
ecommerce-intel-pp-cli explain-drop --profile store <product-or-query>
ecommerce-intel-pp-cli query-revenue --profile store <topic>
ecommerce-intel-pp-cli inventory-risk --profile store
ecommerce-intel-pp-cli restock-winners --profile store
ecommerce-intel-pp-cli cannibalization --profile store
ecommerce-intel-pp-cli category-clusters --profile store
```

## Source adapters

The source adapters cover Shopify, Klaviyo, GA4, GSC, and Ahrefs child CLIs plus local JSON fixtures/import. Use `sources doctor` to confirm which optional binaries and env vars are available before live sync. Credentials and network access remain owned by the child CLIs.

Child CLI JSON must include a supported `schema_version`; unknown or missing child schemas fail closed before data feeds confidence, movers, or later apply surfaces.

Every sync preserves the latest `<profile>-data.json` file and appends a dated snapshot under `snapshots/<profile>/` with schema version, source command versions, date range, and input hashes. Retention keeps daily snapshots for 30 days and weekly snapshots after that. Mover and outcome notes append to `learnings/<profile>.md`.

```bash
ecommerce-intel-pp-cli sync --profile store --source shopify --shop example.myshopify.com
ecommerce-intel-pp-cli sync --profile store --source gsc --site sc-domain:example.com
ecommerce-intel-pp-cli sync --profile store --source all --shop example.myshopify.com --klaviyo-account acct --ga-property 123 --site sc-domain:example.com --ahrefs-target example.com
```

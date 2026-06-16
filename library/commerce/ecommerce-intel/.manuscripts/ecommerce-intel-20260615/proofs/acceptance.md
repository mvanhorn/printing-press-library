# Ecommerce Intel acceptance proof

## Commands verified locally

```text
(cd library/commerce/ecommerce-intel && go test ./...)
?   github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/cmd/ecommerce-intel-pp-cli [no test files]
ok  github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/internal/cli
ok  github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/internal/store
```

```text
(cd library/commerce/ecommerce-intel && make smoke)
agent-context emitted JSON with private=true and source_plan for Shopify, Klaviyo, GA4, GSC, Ahrefs.
doctor emitted readiness without secret values.
sync loaded embedded-shopify-commerce-fixture.
digest weekly reported revenue and GEO score.
geo-audit reported GEO score, check count, and gaps.
```

## Regression checks added

- `opportunities` prints inventory rows with `type=inventory`; no Go `%!s(<nil>)` formatting leak.
- `sync --source nonsense` returns an invalid-source error instead of silently accepting unknown source names.
- `sources doctor` reports env var presence without printing secret values.
- `geo-audit --agent` includes answer-engine fields and llms.txt recommendations.

## CI follow-up

The PR intentionally excludes generated root `registry.json` and generated catalog README changes. The post-merge registry workflow should regenerate them from `.printing-press.json`.

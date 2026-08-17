# Open Food Facts CLI

`open-food-facts-pp-cli` is a read-only Printing Press CLI for Open Food Facts product lookup, structured product search, nutrition summaries, ingredient and allergen inspection, small product comparisons, category samples, and source posture checks. It returns compact `--agent` JSON while preserving source URLs, data-quality caveats, custom `User-Agent` guidance, and anti-bulk limits.

## Sources

- Open Food Facts API introduction: product reads, data-quality caveats, deployments, authentication, and rate limits.
- API v3 product endpoint: current product-read path for new integrations.
- API v2 search endpoint: structured search path for categories, brands, countries, labels, and nutrition grades.
- Open Food Facts data guidance: community-contributed records are not guaranteed accurate, complete, or reliable.

## Setup

No API key is required for read operations.

Open Food Facts asks clients to identify requests with a custom `User-Agent` in the form `AppName/Version (ContactEmail)`:

```bash
export OPEN_FOOD_FACTS_USER_AGENT="my-food-tool/1.0"
export OPEN_FOOD_FACTS_CONTACT_EMAIL="you@example.org"
```

Without explicit configuration, the CLI still sends an identifiable default `User-Agent` naming this CLI and package. For regular or frequent use, configure a contact-bearing identity and stay within Open Food Facts' documented limits.

Direct Go installs require a Go toolchain compatible with the module's `go 1.26.4` declaration.

## Commands

```bash
open-food-facts-pp-cli product 3017620422003 --agent
open-food-facts-pp-cli search --category "breakfast cereals" --country "united-states" --page-size 5 --agent
open-food-facts-pp-cli nutrition 3017620422003 --agent
open-food-facts-pp-cli allergens 3017620422003 --agent
open-food-facts-pp-cli compare 3017620422003 5449000000996 --agent
open-food-facts-pp-cli category "breakfast cereals" --page-size 5 --agent
open-food-facts-pp-cli sources --agent
open-food-facts-pp-cli doctor --agent
```

## Caveats

- Do not use this CLI for product edits, image uploads, account login, or session workflows.
- Do not use it as a bulk product backend. Use Open Food Facts CSV or JSONL exports when fetching more than a few hundred products.
- Product records are community-contributed and may be incomplete or stale.
- This CLI reports nutrition, ingredient, allergen, and data-quality fields from Open Food Facts; it does not provide medical or dietary advice.
- Structured search uses `/api/v2/search` because structured search is not available in v3.

---
name: pp-open-food-facts
description: "Use Open Food Facts source-backed recipes for product lookup, structured search, nutrition summaries, allergen inspection, product comparison, and category samples. Trigger phrases: Open Food Facts, food barcode, nutrition facts, allergens, ingredients, product comparison, open-food-facts-pp-cli."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - open-food-facts-pp-cli
    install:
      - kind: go
        bins: [open-food-facts-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/food-and-dining/open-food-facts/cmd/open-food-facts-pp-cli
---

# Open Food Facts - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `open-food-facts-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install open-food-facts --cli-only
   ```
2. Verify: `open-food-facts-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/open-food-facts/cmd/open-food-facts-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When To Use

Use `open-food-facts-pp-cli` when an agent needs source-backed Open Food Facts data:

- Fetch a product by barcode with source URL, identity, labels, countries, Nutri-Score, NOVA, Eco-Score, nutriments, and data-quality tags.
- Run bounded structured product search by category, brand, country, label, or nutrition grade.
- Summarize nutrition facts for one known barcode.
- Inspect ingredients, allergens, traces, additives, and data-quality tags for one known barcode.
- Compare a small set of barcode records without hiding per-product fetch errors.
- Fetch a small category sample and preserve Open Food Facts data caveats.

## When Not To Use

- Do not use it for product edits, image uploads, account login, or session workflows.
- Do not use it for bulk harvesting or local database sync; use Open Food Facts CSV or JSONL exports for bulk access.
- Do not build search-as-you-type loops against the live API.
- Do not treat nutrition, ingredient, or allergen output as medical or dietary advice.
- Do not claim Open Food Facts data is complete, accurate, or authoritative.

## Setup

No API key is required for read operations.

The module declares `go 1.26.4` so environments with Go toolchain auto-download disabled should use Go 1.26.4 or newer for direct installs.

Open Food Facts asks every API client to send a custom `User-Agent` in the form `AppName/Version (ContactEmail)`. Configure a request identity before regular or frequent use:

```bash
export OPEN_FOOD_FACTS_USER_AGENT="my-food-tool/1.0"
export OPEN_FOOD_FACTS_CONTACT_EMAIL="you@example.org"
```

Optional local/staging override:

```bash
export OPEN_FOOD_FACTS_BASE_URL="https://world.openfoodfacts.org"
```

Keep live usage within Open Food Facts' documented limits: 15 product read requests per minute per IP address, 10 search requests per minute per IP address, and no live API bulk harvesting.

## Recipes

### Product Lookup

```bash
open-food-facts-pp-cli product 3017620422003 --agent
```

Use this when the workflow has a barcode and needs compact product identity, source URL, Nutri-Score/NOVA/Eco-Score fields, ingredient/allergen tags, nutriments, and data-quality caveats.

### Structured Search

```bash
open-food-facts-pp-cli search --category "breakfast cereals" --country "united-states" --page-size 5 --agent
```

Use this for bounded candidate discovery. Prefer structured filters over plain terms because Open Food Facts documents structured search in API v2 and does not provide full-text search in v2/v3 server-side APIs.

### Nutrition Summary

```bash
open-food-facts-pp-cli nutrition 3017620422003 --agent
```

Use this when a workflow needs nutriment fields, serving basis, Nutri-Score, NOVA, Eco-Score, source URL, and data-quality tags for one barcode.

### Allergens And Ingredients

```bash
open-food-facts-pp-cli allergens 3017620422003 --agent
```

Use this to inspect ingredient text, ingredient analysis tags, allergens, traces, additives, and data-quality tags. Keep the caveat that records are community-contributed.

### Small Product Comparison

```bash
open-food-facts-pp-cli compare 3017620422003 5449000000996 --agent
```

Use this for side-by-side comparison of two to five barcodes. The command returns successful products plus per-barcode errors when a lookup fails.

### Category Sample

```bash
open-food-facts-pp-cli category "breakfast cereals" --page-size 5 --agent
```

Use this to fetch a small product sample for a category through structured search. Do not page through many results as a bulk backend.

### Source Coverage

```bash
open-food-facts-pp-cli sources --agent
open-food-facts-pp-cli doctor --agent
```

Use these before larger workflows to confirm read-only posture, optional request identity, configured base URL, documented rate limits, and non-goals.

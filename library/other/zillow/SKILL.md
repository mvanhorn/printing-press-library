---
name: pp-zillow
description: "Zillow Zestimate + deal intelligence CLI. Get Zestimates, rent estimates, tax assessed values, and find properties where Zillow's AVM exceeds the list price. Trigger phrases: `zillow search <city>`, `get zestimate for <address>`, `find zillow deals in <city>`, `compare these zillow listings`, `zestimate vs list price`, `use pp-zillow`."
author: "Kapowsin AI - Callie"
license: "Apache-2.0"
argument-hint: "<command> [args]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pp-zillow
    install:
      - kind: shell
        bins: [pp-zillow]
        command: "go install github.com/kapowsin/printing-press-library/library/other/zillow/cmd/zillow-pp-cli@latest"
        label: "Install via go install"
---

# Zillow — Printing Press CLI

## Prerequisites

This skill drives the `pp-zillow` binary. Verify it is installed before use:

```bash
pp-zillow --help
```

If missing, install from source (requires Go 1.26.3+):
```bash
go install github.com/kapowsin/printing-press-library/library/other/zillow/cmd/zillow-pp-cli@latest
```

Uses Chrome TLS fingerprinting (enetx/surf) to access Zillow data. No API key required.

## What Makes This CLI Unique

Zillow shut down their public Zestimate API in 2021. This CLI restores that access by fetching Zillow's search page HTML with Chrome browser impersonation, extracting Zestimate, rentZestimate, and taxAssessedValue from the embedded page state.

**The deal signal:** Properties where `zestimate > list_price` indicate potential underpricing. The `deals` command surfaces these automatically.

**Three-way valuation in one call:**
- List price (what the seller is asking)
- Zestimate (Zillow's AVM)
- Rent Zestimate (estimated monthly rental income)

## When to Use This CLI

Reach for pp-zillow when:
- An agent or investor wants a quick AVM alongside listing price
- Screening for underpriced properties (Zestimate > list price)
- Comparing multiple properties' Zestimates side-by-side
- Supplementing pp-redfin comps with a second independent valuation source

Skip it for: one-off browsing (use zillow.com), MLS listing data (use pp-redfin), historical sold data (use pp-redfin comps).

## Commands

### `search <city>`
List homes with Zestimate for a city. Shows list price, Zestimate, gap %, beds/baths, sqft, days on market, rent estimate.

```bash
pp-zillow search "Tacoma, WA"
pp-zillow search "Federal Way, WA" --limit 10
pp-zillow search "Seattle, WA" --json
```

### `deals <city>`
Find properties where Zillow thinks the home is worth more than the asking price.

```bash
pp-zillow deals "Tacoma, WA"
pp-zillow deals "Pierce County, WA" --gap 20    # Zest > list by 20%+
pp-zillow deals "Seattle, WA" --gap 10 --limit 10
```

**Output includes:** address, list price → Zestimate, gap %, beds/baths, sqft, rent estimate, Zillow URL.

### `zestimate <url or address>`
Get the Zestimate for a specific property by URL or address.

```bash
pp-zillow zestimate https://www.zillow.com/homedetails/.../12345_zpid/
pp-zillow zestimate "3506 S Melrose St Tacoma WA"
```

### `compare <url> [url...]`
Side-by-side comparison of multiple properties including Zestimate, rent estimate, and tax assessed value.

```bash
pp-zillow compare https://zillow.com/... https://zillow.com/...
pp-zillow compare --json https://zillow.com/... https://zillow.com/...
```

## Output Format

All outputs are Telegram-friendly bullet lists. No wide Markdown tables.
Pass `--json` for structured JSON output suitable for piping to other tools.

## Known Limitations

1. Zillow uses Cloudflare. The surf library impersonates Chrome to bypass it, but aggressive bot detection may cause occasional 403s. Retry after 30-60 seconds.
2. Zestimate is an AVM estimate, not a guaranteed value. Accuracy varies by market.
3. Search URL slugs are city-name based (`tacoma-wa`). For unusual city names, try the `--json` flag and verify addresses in output.
4. Data is from Zillow's search page state, which may not match individual property detail pages for off-market or recently sold homes.

## Author

Built by Kapowsin AI - Callie (Kapowsin Business Solutions LLC)
Contributing to the Printing Press community library.

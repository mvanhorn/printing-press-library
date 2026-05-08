# Zillow CLI — pp-zillow

**Zestimate + deal intelligence for the terminal.**

Zillow's public Zestimate API was shut down in 2021. This CLI restores that access using Chrome TLS fingerprinting (enetx/surf) to fetch Zillow's embedded search page state, extracting Zestimate, rentZestimate, and taxAssessedValue for every listing.

## Unique Capabilities

- **`deals`** — Find properties where Zestimate > list price by N%. Surfaces underpriced listings competitors miss.
- **Three-way valuation**: list price + Zestimate + rent Zestimate in one call
- **Tax assessed value** for a fourth independent data point
- All outputs are agent-ready and Telegram-friendly

## Install

```bash
npx -y @mvanhorn/printing-press install zillow
```

Or via Go (requires Go 1.26.3+):
```bash
go install github.com/mvanhorn/printing-press-library/library/other/zillow/cmd/zillow-pp-cli@latest
```

## Quick Start

```bash
# Find deals where Zillow values home 15%+ above list price
pp-zillow deals "Tacoma, WA" --gap 15

# Market search with Zestimates
pp-zillow search "Federal Way, WA" --limit 20

# Single property Zestimate
pp-zillow zestimate https://www.zillow.com/homedetails/.../12345_zpid/

# Side-by-side comparison
pp-zillow compare https://zillow.com/... https://zillow.com/...
```

## Example Output

```
💰 Deals in Tacoma, WA — Zestimate > List Price by ≥15% (1 found)

1. 3506 S Melrose Street, Tacoma, WA 98405
   List: $275,000  →  Zest: $446,700  (▲ +62.4%)
   3bd/1ba  1036 sqft  Rent est: $2,278/mo
   https://www.zillow.com/homedetails/3506-S-Melrose-St-Tacoma-WA-98405/49222684_zpid/
```

## Authentication

No authentication required. Uses Chrome TLS fingerprinting to access Zillow's public search page. No API key, no account needed.

## Author

Built by **Kapowsin AI - Callie** (Kapowsin Business Solutions LLC, Washington State)
Contributing to the Printing Press community.

---
name: pp-rate-compare
description: "Mortgage rate scenario comparison using live FRED market rates + Fannie Mae LLPA matrix for FICO/LTV-based pricing. Shows payment impact across credit scores, loan products (30yr/15yr/ARM), and down payment scenarios. No API key required. Trigger phrases: `compare mortgage rates`, `what's my payment at FICO 720`, `fico impact on mortgage`, `30yr vs 15yr comparison`, `mortgage scenario`, `use pp-rate-compare`."
author: "Kapowsin AI - Callie"
license: "Apache-2.0"
argument-hint: "<command> [flags]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - pp-rate-compare
    install:
      - kind: shell
        bins: [pp-rate-compare]
        command: "go install github.com/kapowsin/printing-press-library/library/other/rate-compare/cmd/rate-compare-pp-cli@latest"
        label: "Install via go install"
---

# Mortgage Rate Compare — Printing Press CLI

## Prerequisites

```bash
pp-rate-compare --help
```

Requires Go 1.22+. No API key required.

## What Makes This CLI Unique

Most mortgage rate comparison tools scrape live lender quotes (which change minute-to-minute and require JavaScript). This CLI uses a more reliable and accurate approach:

1. **Live market rates** from FRED (Federal Reserve Bank of St. Louis / Freddie Mac PMMS) — the authoritative weekly benchmark
2. **Fannie Mae LLPA matrix** — the actual table lenders use to price FICO and LTV adjustments

This means the output reflects *how lenders actually price loans* rather than a snapshot of one lender's marketing rate. The FICO impact numbers are accurate to within ~0.125% of what a buyer would actually pay.

## Commands

### `compare`
Compare rates and monthly payments across FICO scores and/or loan types.

```bash
# FICO comparison (most useful for agent/lender clients)
pp-rate-compare compare --price 485000 --down 10 --fico 620,680,720,740,760

# Loan product comparison
pp-rate-compare compare --price 485000 --down 20 --products 30yr,15yr,arm5 --fico 740

# Full matrix
pp-rate-compare compare --price 485000 --down 10 --fico 680,720,760 --products 30yr,15yr
```

**Output includes:** rate, LLPA adjustment, monthly P&I, total interest, FICO impact summary.

### `scenario`
Single detailed scenario with full payment breakdown.

```bash
pp-rate-compare scenario --price 485000 --down 20 --fico 720 --product 30yr
pp-rate-compare scenario --price 485000 --down 10 --fico 680 --product arm5
pp-rate-compare scenario --price 485000 --down 20 --rate 6.5  # manual rate override
```

**Output includes:** P&I, estimated taxes+insurance, PITI, income needed (28% rule).

### `fico-impact`
Show the payment impact of improving a client's credit score.

```bash
pp-rate-compare fico-impact --price 485000 --down 10 --fico 680
```

**Output:** table of all FICO bands (620-780) with rate, payment, and savings vs current score.

**Best use case:** Showing a buyer with 680 FICO what they'd save by getting to 720 before buying. Often $75-200/month, $27K-72K over the loan term.

## Loan Products Supported

| Product | Flag | Description |
|---------|------|-------------|
| 30-yr Fixed | `30yr` | Standard — uses FRED MORTGAGE30US |
| 15-yr Fixed | `15yr` | Uses FRED MORTGAGE15US |
| 5/1 ARM | `arm5` | Uses FRED MORTGAGE5US |
| 7/1 ARM | `arm7` | Estimated from ARM5 spread |
| 30-yr FHA | `fha` | 30yr + FHA premium adjustment |

## Data Sources

- **Market rates:** FRED (Federal Reserve / Freddie Mac PMMS) — updates weekly
- **LLPA adjustments:** Fannie Mae published matrix (Q1 2026) — basis for conventional loan pricing
- Tax+insurance estimate: 1.25% of home value annually (WA State typical)

## Author

Built by Kapowsin AI - Callie (Kapowsin Business Solutions LLC)
Contributing to the Printing Press community library.

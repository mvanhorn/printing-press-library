# Rate Compare CLI — pp-rate-compare

**Mortgage rate scenario comparison using live FRED rates + Fannie Mae LLPA matrix.**

Shows exactly how FICO score and LTV affect mortgage rate pricing — using the actual mechanism lenders use (Loan Level Price Adjustments), not scraped quotes. Combined with live Freddie Mac weekly rates from FRED.

## Why This Approach

Most rate comparison tools scrape lender websites (fragile, JavaScript-dependent, quote-of-the-moment). This CLI uses the underlying pricing mechanism:

1. **Live market rate** from FRED (Freddie Mac PMMS — the benchmark all lenders use)
2. **Fannie Mae LLPA matrix** — the published table of FICO/LTV adjustments that determines what a borrower actually pays

The result is more accurate, more stable, and more educational than scraped quotes.

## Install

```bash
npx -y @mvanhorn/printing-press install rate-compare
```

Or via Go (stdlib only):
```bash
go install github.com/mvanhorn/printing-press-library/library/other/rate-compare/cmd/rate-compare-pp-cli@latest
```

## Quick Start

```bash
# Compare FICO scores side-by-side
pp-rate-compare compare --price 485000 --down 10 --fico 620,680,720,740,760

# Compare loan products
pp-rate-compare compare --price 485000 --down 20 --products 30yr,15yr,arm5 --fico 740

# Full payment breakdown
pp-rate-compare scenario --price 485000 --down 20 --fico 720

# Show dollar impact of improving credit score
pp-rate-compare fico-impact --price 485000 --down 10 --fico 680
```

## Example Output

```
📊 Mortgage Rate Comparison — Home: $485,000 | Down: 10% | Loan: $436,500 | LTV: 90%
   Market baseline: 30yr 6.37% | 15yr 5.72% | 5/1 ARM 6.06%

Product         FICO   Rate     LLPA     Monthly P&I   Total Interest
────────────────────────────────────────────────────────────────────────
30-yr Fixed     620    9.12%    +2.75%   $3,550        $841,476
30-yr Fixed     680    7.87%    +1.50%   $3,163        $702,329
30-yr Fixed     720    7.37%    +1.00%   $3,013        $648,291
30-yr Fixed     740    7.12%    +0.75%   $2,939        $621,651
30-yr Fixed     760    6.87%    +0.50%   $2,866        $595,273

💡 FICO Impact (760 vs 620): +2.25% rate = $684 more/month = $246,240 more over 30 years
```

## Use Cases

- **RE agents**: Show buyers the dollar cost of their credit score before they apply
- **Mortgage lenders**: Scenario modeling for client consultations
- **Investors**: Compare 30yr vs 15yr vs ARM for cash-flow analysis

## Author

Built by **Kapowsin AI - Callie** (Kapowsin Business Solutions LLC, Washington State)
Contributing to the Printing Press community.

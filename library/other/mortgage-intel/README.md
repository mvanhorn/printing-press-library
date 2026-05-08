# Mortgage Intel CLI — pp-mortgage-intel

**Live mortgage rates from Freddie Mac via FRED. Free, no API key.**

Fetches the Freddie Mac Primary Mortgage Market Survey (PMMS) weekly rate data from the Federal Reserve Bank of St. Louis (FRED) public API. No authentication required.

## Unique Capabilities

- Live 30yr, 15yr, and 5/1 ARM rates updated weekly
- Weekly change direction (▲ rising / ▼ falling / → flat)
- 4-week trend with rate lock/float guidance
- Affordability calculator with income requirement (28% rule)

## Install

```bash
npx -y @mvanhorn/printing-press install mortgage-intel
```

Or via Go (stdlib only, no external dependencies):
```bash
go install github.com/mvanhorn/printing-press-library/library/other/mortgage-intel/cmd/mortgage-intel-pp-cli@latest
```

## Quick Start

```bash
# Current rates with weekly change
pp-mortgage-intel current

# 4-week trend with guidance
pp-mortgage-intel trend

# Payment calculator
pp-mortgage-intel afford --price 485000 --down 10

# Historical rates (24 weeks)
pp-mortgage-intel history --weeks 24
```

## Example Output

```
📊 Mortgage Rates — Week of 2026-05-07
Source: Freddie Mac PMMS via FRED (free, no API key)

• 30-yr Fixed: 6.37%  ▲ +0.07 pts vs last week
• 15-yr Fixed: 5.72%  ▲ +0.08 pts vs last week
• 5/1 ARM:     6.06%
```

## Data Source

FRED (Federal Reserve Bank of St. Louis) — `MORTGAGE30US`, `MORTGAGE15US`, `MORTGAGE5US` series.
Data from Freddie Mac's Primary Mortgage Market Survey, updated every Thursday.

## Author

Built by **Kapowsin AI - Callie** (Kapowsin Business Solutions LLC, Washington State)
Contributing to the Printing Press community.

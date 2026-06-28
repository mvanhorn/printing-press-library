# AGENTS.md — using biz-insurance-finder programmatically

This CLI is agent-native. Every command emits JSON and never blocks on input
when run with `--agent`.

## One-shot recipes

```bash
# Build once
go build -o biz-insurance-finder ./cmd/biz-insurance-finder-pp-cli

# Rank providers for an existing profile (JSON)
biz-insurance-finder --agent --profile ./acme.json match

# Full plan (warnings + per-provider URL + answer sheet + checklist) as JSON
biz-insurance-finder --agent --profile ./acme.json guide --top 3

# Answer sheet for one provider
biz-insurance-finder --agent --profile ./acme.json answersheet insurance-canopy
```

`--agent` expands to `--json --compact --no-input --no-color --yes`.

## Creating a profile without prompting

`intake` under `--no-input`/`--agent` writes a profile of defaults without
prompting. The faster path for an agent is to write `profile.json` directly —
it is plain JSON. The field shape is the `Profile` struct in
[`internal/insurance/types.go`](internal/insurance/types.go). Minimum useful
fields:

```json
{
  "legal_name": "Acme Imports LLC",
  "entity_structure": "LLC",
  "formation_state": "Delaware",
  "business_address": "<street, city, ST ZIP>",
  "contact_name": "Jane Doe",
  "contact_email": "owner@example.com",
  "contact_phone": "(312) 555-0100",
  "year_started": 2023,
  "revenue_band": "$100k-$175k",
  "industry_class": "Consumer-goods importer",
  "importer": true,
  "private_label": true,
  "manufacturer": true,
  "countries_of_origin": ["China"],
  "gl_per_occurrence": "$1,000,000",
  "gl_aggregate": "$2,000,000",
  "products_completed_ops": true,
  "effective_date": "2026-06-29"
}
```

## Exit codes

`0` ok · `2` usage · `3` not found · `5` data error · `10` config error.

## Hard boundaries (do not cross)

The tool intentionally does **not** act in a browser. An agent driving this tool
must still treat these as human-only steps and stop for the user:

- CAPTCHAs / "I'm not a robot"
- account creation / passwords
- EIN / SSN / government IDs
- payment / card / bank info
- the final **submit** (two-gate: show the values, the human clicks submit)

`biz-insurance-finder checklist <id>` returns these per provider as JSON.

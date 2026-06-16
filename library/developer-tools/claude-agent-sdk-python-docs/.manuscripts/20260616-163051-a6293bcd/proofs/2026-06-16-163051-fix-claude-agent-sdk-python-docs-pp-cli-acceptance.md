# Acceptance Report: claude-agent-sdk-python-docs

Level: Full Dogfood
Tests: 84/84 passed
Skipped: 49 optional or non-applicable checks
Gate: PASS

## Fixes applied
- Rendered binary Markdown `pages` responses as structured JSON envelopes when JSON/agent/select output is requested.
- Returned non-zero not-found errors for invalid docs topics, symbols, examples, guides, recipes, and pages.
- Updated research narrative so regenerated README/SKILL prose matches implemented live-docs behavior.

## Printing Press issues
- None requiring a retro blocker. Dogfood warnings remain around generated dead helpers and the docs CLI's intentional live Markdown corpus path.

# Robinhood API Map Draft

Status: draft seed map, not implementation-complete.

## Contents

- `openapi/robinhood-crypto.openapi.json` — official Robinhood Crypto Trading API OpenAPI extracted from `docs.robinhood.com/crypto/trading/` on 2026-05-22.
- `openapi/robinhood-crypto.openapi.yaml` — YAML conversion of the same official spec for tooling that prefers YAML.
- `openapi/robinhood-brokerage-seed.openapi.json` — normalized OpenAPI 3.1 seed map generated from the brokerage route inventory. It keeps route templates, conservative risk labels, and multi-host observations under `x-robinhood-*` extensions.
- `brokerage-routes.json` — classified seed route inventory from `jmfernandes/robin_stocks`.
- `markdown/brokerage-routes.md` — human-readable brokerage route reference.
- `curl/brokerage-route-templates.sh` — commented curl template file. It is intentionally non-executable guidance and must not be treated as an auth-ready script.

## Current Counts

- Official Crypto API: 14 paths, 16 operations.
- Brokerage seed routes: 69 templates.
- Brokerage seed OpenAPI: 63 normalized paths / 63 operations.
- Brokerage risk split:
  - `read`: 33
  - `sensitive-read`: 24
  - `write-or-sensitive`: 8
  - `destructive`: 4

## Boundaries

This draft is read-only research. It does not prove authenticated brokerage coverage.

Before any route becomes a shipped CLI command:
- verify the live route shape through docs, source, or browser/API sniffing;
- classify risk conservatively;
- keep writes dry-run by default;
- require explicit live confirmation for trades, ACH, unlink, recurring, watchlist mutation, and document download;
- avoid referral links in setup/auth/docs paths per the final goal-doc addendum.

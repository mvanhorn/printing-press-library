# Acceptance Report: etsy (etsy-pp-cli)

- **Level:** Full Dogfood. The binary-owned live matrix ran `cli-printing-press dogfood --live --level full`. Results: `20261117-093252-dogfood-results.json`. Marker: `phase5-acceptance.json`, status pass.
- **Credentials:**
  - the operator's Etsy app key plus shared secret;
  - no OAuth login, and the operator has no shop;
  - shop-scoped public endpoints were exercised against a public sample shop (referred to here only as "the sample shop").
- **Tests:** 171 of 171 executed rows passed (0 failed); 557 were skipped.
  - Skips are mostly underivable required inputs (404), OAuth-only scopes, and mutating commands. Writes were not approved: there is no shop or sandbox, so destructive probes were off.
  - 4 rows are `unverified-needs-access`: `user-addresses get` and `users get-me`, which need OAuth.

## Coverage
- **Flagship and auth:**
  - `doctor` and `sync` passed happy_path and json_fidelity.
- **Novel features:** 18 of 24 passed happy_path and json_fidelity live.
  - Shop reports: listings audit, listings expiring, listings dead-weight, inventory audit, inventory low-stock, money fees, money reconcile, money summary, shop attention, reviews digest, orders unshipped.
  - Market commands: market snapshot, market trend, market demand, market shops, market tags, market price.
- **blocked-fixture or skipped:**
  - `listings bulk-edit` and `market gaps` take a local input file (`--plan`, `--rules`), so they are declared `pp:happy-args` placeholders.
  - `listings copy`, `listings snapshot` and `inventory set-qty` need a real listing id plus write/OAuth access.
  - `search` happy_path needs a query fixture; its error_path and dry_run passed.

## Failures (run 1, fixed)
- `listings bulk-edit --plan audit-fixes.json` (happy_path and json_fidelity): expected a preview, got exit 2, because the input file is absent from the runner sandbox.
- `market gaps --rules classes.yaml --control known-ids.txt` (happy_path and json_fidelity): same cause.

## Fixes applied: 1
- **CLI fix, commit 6e4aaea:** declare `pp:happy-args` placeholders `--plan=example-value` and `--rules=example-value`, so the rows are blocked-fixture per 11-build-the-goat.md. No behaviour change.

## Printing Press issues: 1
- The dogfood runner cannot materialize file fixtures for file-input flags, so file-input novel commands get no live happy-path coverage. Logged in workshop `issues-encountered.jsonl` as a retro candidate.

## Gate: PASS

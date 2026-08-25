# Final review findings

## Review structure

Independent correctness, security, and reliability reviewers examined the generated and hand-built behavior. Three normal fix rounds did not converge, so the user explicitly selected an additional repair round. That round fixed the remaining persistence migration, cleanup convergence, reconciliation, service-allocation, webhook-receipt, and request-readiness issues. Focused re-review passed.

## Result

- Correctness: PASS after extra repair round.
- Security/privacy: PASS for Square-specific additions after secret scrubbing, private-file handling, strict origin classification, bounded request bodies, and no-send request readiness.
- Reliability: PASS after fully convergent history/receipt retention and Windows-safe storage/migration handling.
- Novel workflows: 6 planned, 6 implemented, none missing.

## Upstream retro candidates

- Cross-host redirects should strip all sensitive configured headers, not only `Authorization`.
- Generic MCP SQL should impose a database-level bound before materializing results.
- Generated documentation should normalize or remove unsupported `entity:` and `api-endpoint:` pseudo-links.

These findings are template-shaped and should be addressed in Printing Press itself rather than patched differently into one generated CLI.

## Semantic output review

The independent six-sample output review returned WARN with no blockers. It identified contradictory aggregate `valid` wording in request readiness and unstable `null` versus `[]` collection shapes. Both findings were forwarded into the mandatory polish pass for correction and re-verification.

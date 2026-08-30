# iFood CLI shipcheck

- Run: `20260814-042201-817ea5ed`
- Printing Press: `4.30.2`
- Full live dogfood: 113/113 evaluated checks passed, 0 failed, 100%, no hollow coverage
- Publish validation: pass; the pre-package module-path warning is expected and is rewritten by `publish package`
- Scorecard: 89, grade A; five embedded flagship samples passed
- Remaining unverified dimension: direct private-API `live_api_verification`

## Fixes applied

- Raised `golang.org/x/text` from 0.38.0 to 0.39.0 after reachable `GO-2026-5970` was reported.
- Adapted preserved iFood commands to the current error-classification signature.
- Added credential-free embedded examples for quote validation, cart planning, cross-market quoting, and cart building.
- Made session imports deterministic and credential-free under dry-run, and classified local feedback/session writes accurately.
- Split the cart boundary: `cart build` is strictly read-only; only `cart add --execute --yes` can update an existing cart.
- Corrected MCPB auth configuration to prompt for sensitive `IFOOD_BEARER_AUTH` instead of a generic client profile.
- Corrected README, SKILL, manifest, root help, and MCP descriptions to describe the confirmation-gated write boundary accurately.
- Added narrow security-analysis suppressions for operator-selected local input files; no hand-authored gosec findings remain.

## Validation

- `go test -count=1 ./...`: pass
- `go vet ./...`: pass
- `go build ./cmd/ifood-pp-cli`: pass
- `govulncheck ./...`: no reachable vulnerabilities
- `gosec`: 0 unresolved findings in hand-authored iFood files; 29 findings remain exclusively in generated runtime files and are Printing Press retro candidates
- `verify-skill`: pass, 34 recipes checked, 0 findings
- `validate-narrative --strict --full-examples`: 6/6 pass
- full live dogfood: pass, 113/113, no hollow coverage
- workflow verify: pass
- tools audit: no pending findings; two concise generated framework descriptions accepted with individual rationale
- PII audit: no findings
- output review: pass, five eligible samples, no plausibility findings
- publish validate: pass

## Known limitation

The direct private iFood API was not exercised with an exported bearer token, so the scorecard intentionally leaves `live_api_verification` unverified. A signed-in Browser session was verified read-only against a real 4.9-rated market and live product results, while credentials remained inside the browser. This limitation does not weaken the credential-free examples or the passed full acceptance gate.

No checkout, payment, order submission, address/account change, or cart mutation occurred.

## Final ship recommendation

`ship`

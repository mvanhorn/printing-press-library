# servicetitan-pricebook-pp-cli — Phase 5 Acceptance Report

  Level: Full Dogfood
  Tests: 128/128 passed (162 skipped — positional-arg / mutation commands the matrix conservatively does not exercise)
  Verdict: PASS
  Gate: PASS

## Failures fixed inline (1)
- **`find` error-path** — dogfood ran `find __printing_press_invalid__` and expected a non-zero exit, but `find` returned weak ~0.36-score fuzzy junk and exited 0. Root cause: `find` had no relevance floor — it returned the top-N by score regardless of how weak. **CLI fix:** added a `--min-score` flag (default 0.4) to `find` and `pricebook.Find`; results below the floor are dropped, and a query that matches nothing exits non-zero with an actionable message (grep semantics). Re-verified: nonsense query → exit 1, real queries (`submersible pump`, `1 hp submersible pump motor`) → exit 0 with relevant results. This is a genuine UX improvement, not just a test fix — a "describe the part" finder should not return noise for a nonsense query.

## Behavioral correctness (verified live against the JKA ServiceTitan tenant)
Beyond the 128-test mechanical matrix, the flagship novel commands were spot-checked against real data during Phase 3 and re-confirmed:
- `health --json` — full audit rollup: 100 materials / 100 equipment / 100 services / 89 categories / 1 markup tier; 199 markup-drift, 29 vendor-part gaps, 100 warranty issues, 286 orphan SKUs, 28 duplicate clusters, 200 cost-history rows.
- `markup-audit` — `DR6B Casing Shoe` flagged: actual markup 165% vs tier 100%, delta +65%, expected_price 130 — arithmetic correct.
- `warranty-lint` — flagged `H2PL82` (manufacturer warranty has duration but no description) and `FECB0501IND` (description not prefixed "Manufacturer's") — exactly the JKA attribution rules.
- `vendor-part-gaps` — flagged `DR6B` (primary vendor Foremost Industries, blank vendor part).
- `sync` — pulled 426 records across 7 resources; composed-auth (ST-App-Key + OAuth2 bearer) and tenant substitution confirmed end-to-end.

## Printing Press issues (for retro)
- **scorecard `--live-check` executability probe has no Windows `.exe` code path** — reports a valid `.exe` as "not executable", which blanked the Phase 4.85 agentic output review (forced SKIP) and the scorecard Sample Output Probe.
- **v4.6.1 generator did not wire the apiKey half of composed auth, the sync resource registry, or `{tenant}` substitution** for the ST module spec — the #1303/#1305/#1332 carry-forward; patched from the sibling template.
- **`x-mcp.endpoint_tools: hidden` is not propagated to the cobratree MCP mirror** — the typed endpoint-mirror tools are suppressed but the cobratree walker still re-exposes all 40 endpoint commands, producing `mcp_token_efficiency 0/10`.
- Phase 5 acceptance marker recorded `auth_context.type: "bearer_token"` for a composed apiKey+bearer API — the matrix infers auth from the CLI's runtime self-report rather than the spec; cosmetic, downstream of the composed-auth shape.

## Gate: PASS — proceed to Phase 5.5 (Polish)

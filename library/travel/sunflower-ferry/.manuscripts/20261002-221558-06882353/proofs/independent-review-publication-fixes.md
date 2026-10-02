# Targeted publication-fix review

Current verdict: **PASS — both follow-up findings are fixed and independently verified.** The initial reproduction record below is retained; the final verification section closes it.

Reviewed 2026-10-02T18:01:54.303082+00:00 by the existing authorized reviewer. No extra agents, live provider samples, browser/CDP sessions, source edits, bookings/holds/accounts/payments, or GitHub writes were used.

The Beppu privacy/capacity regressions and terminal box reorder/unknown/duplicate tests pass, and README manual MCP installation correctly installs both companion binaries. The managed publication copy matches operations.go and README exactly; its test file differs only by the expected module import rewrite.

The initial check found two actionable issues in the new implementation:

1. **P2 — shared cabins on other routes become private.** `internal/ferry/operations.go:459–468` defaults to private and only recognizes Room Type text or tourist names. Paragraph-based rows pass an empty Room Type at line 505. Actual retained Kobe `Private bed / All seats reserved` and Shibushi `Private Single / 1 person by each section` and `Private Bed (11room) / 1 person by each section` therefore become private. Preserve these explicitly shared/known headings without substring matching across descriptions or reclassifying Beppu private group4/6. Add Kobe/Shibushi fixture cases.

2. **P2 — unknown terminal names still match known identities.** `internal/ferry/operations.go:643–651` uses substring matching. `Osaka Terminal10` matches Terminal1; `Not Oita Port` matches Oita. Match normalized known aliases exactly, or enforce token/numeric boundaries and reject unrecognized extras, then add these negative cases alongside existing reordered/duplicate cases.

Independent proof: `reviewer-publication-targeted.json`. Two additional deterministic checks were compiled through a Go overlay whose test file and mapping live only in this proof directory; project files were not edited. They reproduce all three private-class regressions and both terminal false matches. The overlay run exits 1; this is intentional failing regression evidence, not a tool/network failure or live result. Source fixtures serve only as deterministic retained-source evidence.

At the initial check, publication verification was pending these two small corrections. Earlier safety, provider, MCP package and generator-limit review conclusions are unchanged.

## Final correction verification — 2026-10-02T18:05:12.062550+00:00

**PASS — both targeted follow-up findings resolved; no new finding in this source check.**

The current managed publication code recognizes explicit section/per-compartment capacity wording and the exact known Kobe Private bed/All seats reserved combination as dormitory. Source-derived Kobe/Shibushi assertions retain shared privacy and omit invented room occupancy bounds. Actual Beppu private group4/6 and semi-private twin1–2/2 regressions still pass.

Terminal matching now uses only exact normalized observed headings for the six verified port identities. The same independent negative cases no longer accept Terminal10 or Not Oita Port. Canonical ordering, reversed source boxes, unknown names and duplicates remain covered across all three routes.

The exact same reviewer overlay that previously reproduced these regressions now passes against the managed publication source, together with the added privacy, port and retained-source tests. Evidence: `reviewer-publication-verified.json` (exit 0). Operations and README match the synchronized original project; the test module import rewrite is expected. The manual MCP section installs both CLI and MCP and states why the companion is required.

Reviewed managed-file SHA-256 values:

```json
{
  "internal/ferry/operations.go": "5be353e85e9d322ecf534692fc90991102eccc9f9d25652a89ec0ce8bde2b4fb",
  "internal/ferry/ferry_test.go": "44f8a63521ebe8a70a6812419299ba79db31a23bb92f91ada256cdd6b5430ea1",
  "README.md": "f97d47f8d61ce6519d9c381efb1defc2c83248c124140837ba07e9b89c7e9f64"
}
```

No provider samples were rerun, project source was not edited by the reviewer, and no GitHub write was performed. The builder's fresh source-bound full acceptance and final push remain runner/owner work; this proof does not synthesize their results. Earlier generator limitation/security dispositions remain intact.

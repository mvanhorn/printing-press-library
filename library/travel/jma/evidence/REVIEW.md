# Independent review

Reviewer: exactly one fresh-context `gpt-6.1-sol`, xhigh, no inherited conversation, no edits. Baseline: empty project; all source new. Reviewer independently inspected source contracts, ran domain/CLI tests and real CLI/MCP calls from isolated caches/homes. It returned ten actionable findings; builder fixed all and reviewer independently reverified each behavior.

| Severity | Original location | Finding | Verification |
|---|---|---|---|
| P1 | forecast.go:114 | Representative short region treated as exclusive weekly coverage | Live north Izu returns eligible broader Izu region and weekly station |
| P1 | forecast.go:240 | Morning extrema indices and corrected-evening issue bucket wrong | Synthetic supported-contract morning replay and evening bucket tests pass |
| P1 | mcp/tools.go:41 | Typed MCP endpoints bypassed domain resolution/coherent detail | Live MCP now routes through normalized companion CLI |
| P2 | warnings.go:113 | Inland offices falsely required wave product | Live Kofu with source applicability reports no weather warnings |
| P2 | warnings applicability branch | Unknown lifted hazard hidden by initial exemption fix | Synthetic unknown record stays visible/incomplete |
| P2 | warnings.go:85 | Unknown downgrade strings treated as known | Exact source status whitelist; unknown transition incomplete |
| P2 | jma_commands.go projection | Valid field projection failed on empty arrays | Empty live index/search projection remains []; typo still fails |
| P2 | typhoon point construction | Missing scale/intensity became literal `<nil>` | Live normalized points now null |
| P2 | doctor replacement | Generated platform conformance test bypassed | Platform doctor preserved when adapter registered; suite passes |
| P3 | SKILL.md example | Invalid Kyoto municipality ID | Live canonical 2610000 resolves |

Reviewer final report: all behavioral fixes independently verified; no further behavioral findings. The obsolete MCP test expecting the previous raw/local framework context was replaced with a consequential cross-surface registration test; full-suite final evidence records its result. Final verification requested from the same reviewer after that update and command-file extraction; see REVIEW-FINAL.md.

Synthetic parser regressions are explicitly separate from live proof. Live findings came from real JMA responses; no guessed fixture result was presented as live.

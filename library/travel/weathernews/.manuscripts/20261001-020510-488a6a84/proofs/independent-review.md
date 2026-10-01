# Independent code review

Exactly one fresh-context reviewer: gpt-6.1-sol, xhigh, fork_turns none. No edits or additional agents. Reviewed project, agreed requirements, whole-project new baseline, hand-authored diff and live/testing/Press evidence. Reused same reviewer for verification.

Initial independently verified findings (all fixed):
- P2: generic projection allowed invalid selectors around empty arrays/mixed selectors. Fixed with command-specific domain shape validation and projection. Unknown paths return exit 2 before output, valid empty collections/null parents preserved.
- P2: missing/invalid source geo, JCODE, place URL and coverage discriminator could yield invented identity. Source identity validation now fails with exit 5; provider-declared is_japan=0 remains exit 3.
- P2: explicit prior-year seasonal dates were rewritten to current title year. Explicit years now must agree; mismatches remain unavailable and cannot pass comparisons.
- Follow-up P3: selected CSV/plain headers included unselected keys. Selected declared header paths retained after projection.

Identity/year reproductions used synthetic mutations of actual live cached bodies; they are source-drift tests, not observed live provider failures. Projection bug was live reproduced.

Final reviewer conclusion: “No remaining independently verified findings. All three original P2 findings and the subsequent P3 formatting regression are fixed.” Reviewer verified original caches, all six output schemas, fresh live Kyoto forecast and koyo detail, and passing evidence/CLI/MCP tests. No source/config edits by reviewer.

Final preserved source-helper metadata fixes (fixture delimiters and clean JSON dry runs) were verified by the same reviewer with fresh live helper calls and CLI tests. Final clearance: no remaining independently verified findings.

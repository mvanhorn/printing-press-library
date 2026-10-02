# Acceptance Report: sunflower-ferry

Level: Full Dogfood. Runner status PASS; 120/120 mandatory checks passed, 0 failed. The JSON also retains 94 skipped/unverified cases; these are not counted as exercised. Tests include every approved feature with real source arguments, CLI help and JSON/dry-run fidelity.

One fix loop: the first pass had 116/117, with only missing routes list help Examples. Added the concrete routes list --agent example, rebuilt CLI/MCP/MCPB, and reran the actual full matrix. The current source-bound phase5-acceptance.json was written by the Press runner, never hand-authored or edited. The same independent reviewer verified the final help-only package change in independent-review-round3-help.md, with zero additional provider requests.

The reviewer independently assessed all six novel live samples, six directions, car/child, motorcycle, toddler/infant, E midnight crossing and calendar year rollover. A real January 3 quote response explicitly states reservations are not yet open and cannot display availability; the CLI returns exit 5 without inventing a sailing/fare or normal-schedule fallback. No unsupported opening-time reason is asserted.

The three Kansai–Kyushu routes are the verified source scope. Oarai–Tomakomai remains outside this release. Fares are source party/category display values with no arithmetic, no inventory guarantee and missing fuel/tax/fee breakdown. Normal-ticket cancellation/baggage guidance may differ for campaigns, agency and vehicle tickets. No booking holds, cabin selection, personal details, accounts, standby registration, cancellation or payment stage was called.

Generated lower-level raw HTML/body limits, optional HTTP MCP hardening, shared gosec diagnostics and local learning-candidate fixture gaps are disclosed in README/review/security/harness proofs. They do not invalidate the completed native bounded planning scope.

Gate: PASS.

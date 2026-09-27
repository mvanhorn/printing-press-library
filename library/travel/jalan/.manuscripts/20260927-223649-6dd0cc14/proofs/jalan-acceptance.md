# Acceptance: Jalan

Level: full. Gate: PASS.

Binary-owned matrix: 101/101 executed checks passed; 78 framework, mutating, fixture-less or inapplicable checks skipped. Every approved stay feature has a successful happy-path and JSON-fidelity check. All relevant stay help, dry-run and positional-error rows pass; zero hollow approved features. The unedited `phase5-acceptance.json` binds this result to the source fingerprint.

Independent source-checked suite: 37/37 cases, 42 CLI invocations, 16 CLI upstream requests, 10 independent public-source requests (`live-e2e-acceptance.json`). Covers ryokan, onsen, urban hotel, bath negatives, reviews, dated exact plan/room terms, family/two-room occupancy, filters, explicit reference-price fallback/no-matches, pagination, comparison and freshness.

Fixes: help examples and semicolon fixture metadata; top-level dry-run discovery markers retain the original envelope; exact property fixture IDs; encoding-selection regression; explicit cleanup handling. Live-format fixtures now request an isolated fresh observation and permit five-minute reuse for the second format check. Shipping defaults remain fresh. No source validation was relaxed.

Source limitation: intermittent unrecognized offers responses occurred in earlier benchmark/matrix runs. Direct bounded diagnostics did not reproduce them; original failed bodies were not retained. The CLI surfaces explicit parse/partial failures, and no invented no-inventory result. Successful and failed proofs are preserved.

The successful cold/warm measurements cover search, offers, exact plan and two-date comparison across two runs. `efficiency-summary.json` identifies their original proof files and earlier failures. This is not an uninterrupted-all-green benchmark claim.

Generator issues are recorded locally in phase-4.95-findings.md and gosec-triage.json. No external messages, issues, PRs or shared configuration changes were made. Delivery remains local.

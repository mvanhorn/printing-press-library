# Detailed PR review — Government Data

Reviewed the catalog, SEC, persistence, authentication, download, and MCP paths, including failure behavior and generated framework integration. Three independent correctness, security, and maintainability reviews completed three rounds with no remaining actionable findings in the final round.

## Findings fixed

| Priority | Root cause | Correction and regression coverage |
| --- | --- | --- |
| P1 | Fixed federal requests reused credentials selected for a different configured server. | Bind federal credentials to the HTTPS api.gsa.gov origin. Foreign-profile keys are omitted; explicit demo access remains available. Recording-transport tests verify the destination and headers. |
| P2 | Filtered pagination could request more rows than the remaining result budget, discard matching rows, and falsely report completeness. | Request the remaining allowance and reject oversized upstream pages. Tests cover filtered final pages, continuing cursors, and both catalogs. |
| P2 | Snapshot evidence committed before its offline search indexes, consuming immutable IDs on partial failure. | Commit snapshot, membership, typed records, generic records, and search indexes in one transaction. Injected insert/FTS failures verify rollback and successful retry with the same ID. |
| P1 | MCP advertised catalog commands as read-only despite optional or unconditional local database writes. | Distinguish local writes from API reads; expose safe arguments and accurate hints in each mode. Tests inspect actual registered schemas and reject blocked arguments before execution. |
| P2 | MCP hid the required output flag for intake lock, making its advertised tool unusable. | Default the manifest to stdout; retain exclusive file creation for explicit CLI destinations. Tests execute the default and inspect normal/read-only MCP registration. |

The two earlier reviewer findings are also fixed: explicit-profile logout persists a global-credential fallback tombstone without deleting the global key, and token-save output reports the actual profile credential path.

## Root-hook follow-up

Greptile identified a further P1 integration gap: removing unconditional read-only annotations also made the root learning hook open and migrate the store for catalog reads without a save. The root write decision now honors conditional annotations and actual parsed flag values. Regression tests cover find/export with omitted, empty, and populated saves; false/true booleans; and sync. An independent correctness review found no actionable concerns. Real find/export calls with default learning created no database in a disposable home. Full tests, CLI race tests, publish validation, and all 183 executed live checks passed again.

## Verification

- Full Go test suite and go vet passed.
- Race tests passed for config, catalog, snapshot cache, store, and CLI packages.
- Current live matrix: 183 executed checks passed, zero failed; 141 checks were skipped/unverified. These counts do not imply every generated command was exercised live.
- Separate real lifecycle passed: four datasets captured across both catalogs, each source found through typed offline search, and a selected-resource manifest emitted to stdout. Temporary home and database removed; raw API responses were not retained.
- Publish validation passed, including build, module consistency, reachable vulnerability checks, patch metadata, and acceptance proof.

## Limits

This is a detailed engineering review, not a proof of absence of defects. Live results depend on upstream availability and catalog contents. Metadata fingerprints identify upstream metadata; they are not signed attestations, downloaded-content hashes, or protection against a manually altered local database. The older static-analysis triage is historical; it is not represented as a fresh security scan. GitHub checks and automated review must be assessed on the final pushed commit.

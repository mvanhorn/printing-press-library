---
date: 2026-10-03
target_cli: iko-yo-pp-cli
amend_run_id: cache-integrity-20261003
scope_tier: minimal
findings_count: 1
---

# Iko-yo Trip cache correctness amendment

Scope: one confirmed cache correctness finding, including its reader, alias-write and URI boundaries. User-authorized direct input; reuse existing draft PR2254 and the same sole reviewer. No retro or upstream issue was requested.

Before-fix actual command reproduction: with a committed canonical writer still open, cached, local inspect and local compare all returned success with old facts; the updated name was absent. The private log is pipeline/cache-wal-before-fix.log.

Correction: refuse active/ambiguous WAL, shm or rollback-journal state across each saved read; resolve selected aliases and reject multi-link DBs; copy an inode-verified DB descriptor into a private bounded snapshot for actual SQL, eliminating the lazy-open swap/restore seam. Snapshot paths receive the same URI checks and are cleaned on failure. Apply this boundary to source-advertised MCP SQL. Save via the canonical path using one bound ordinary SQLite connection/transaction; reject URI traps and ambiguous hard links before opening/migrating/writing, validate selected identity around save. Deliberate external same-user replacement during writes remains unsupported and detected replacements fail.

Exact dependency repair: modernc.org/sqlite v1.46.2 and modernc.org/libc v1.70.0, bundled SQLite3.51.3. Required transitive changes only. Official WAL-reset reference: https://sqlite.org/wal.html#walreset .

Verification: actual command refusal with active committed writer; canonical sidecars through file/directory symlinks; canonical cooperative writer saves and hard-link live-inspect refusal without loss; two-cache/new-path URI traps; private TMPDIR URI cleanup; actual substituted-row lazy SQL test; SQLite runtime query. Then full tests/vet/vulnerability/live and rebuilt CLI/MCP/bundle, focused same-reviewer signoff, candidate push while draft, current-head automated review/readiness.


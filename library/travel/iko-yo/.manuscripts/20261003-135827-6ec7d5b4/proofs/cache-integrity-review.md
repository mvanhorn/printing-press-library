# Focused cache integrity amendment review

Verdict: **PASS**. No unresolved correctness, privacy or framework-seam findings in this amendment. This is the same sole release reviewer; no second reviewer or source fixes were used.

Reviewed the uncommitted cache amendment on base HEAD `23e2804d66dac99272715bb72af57d5e89abc36d` in `library/travel/iko-yo`. This signoff covers the 17 literal source/document/dependency/patch hashes in [cache-integrity-snapshot.json](cache-integrity-snapshot.json) and the final candidate, installed and bundled peers recorded there. Earlier review reports and snapshots remain original evidence and do not cover the changed production source.

## Source and behavior

- `internal/cacheguard/guard.go:27,38,67,171,203`: reads validate selected and canonical paths, regular-file identity and link count, refuse active/ambiguous canonical or selected WAL/SHM/journal sidecars, and copy at most 128 MiB through a verified pinned descriptor into a private 0700 directory with a 0600 file. Identity and sidecar checks surround reads. Selected/resolved URI characters are rejected before SQLite; canonicalized temporary paths are checked and failed clones are cleaned.
- `internal/cli/iko_yo_trip_helpers.go:33,83,105`, `trip_cached.go` and `trip_compare.go:88`: normal writes use the canonical path and normal SQLite transactions. Saved reads use the private snapshot; validation errors discard facts before successful output. The comparison-level guard also prevents a mixed successful result when the selected cache changes.
- `internal/mcp/iko_yo_trip_sql.go:19` and `iko_yo_trip_surface.go:15`: the source-owned SQL override changes source opening while retaining the generated read-only query validator, 5-second query deadline, 4 MiB value cap, row scan limits and SQL result envelope. Deferred row/connection/store closure precedes final original-source validation and snapshot cleanup. Reserved/generated framework files are unchanged by this amendment.
- `README.md:237` and `SKILL.md:139` describe the implemented stable-cache reader contract, canonical writers, alias/URI limits and recovery instruction. Deliberate external same-user inode swap/restore during writer open remains outside the supported writer contract; detected retargets error. I found no supported command path that closes a raw source descriptor beside a same-process live SQLite writer: Trip MCP writes shell out to the CLI, and saved CLI readers follow closed stores.
- `go.mod:13,27` pins modernc/sqlite 1.46.2 and libc 1.70.0. The actual candidate MCP reports SQLite 3.51.3. The [official SQLite 3.51.3 release notes](https://sqlite.org/releaselog/3_51_3.html) identify its WAL-reset fix.

## Independent verification

Twelve focused top-level Go tests passed across `internal/cacheguard`, `internal/cli` and `internal/mcp`, using this scoped test selection:

```text
go test ./internal/cacheguard ./internal/cli ./internal/mcp -run 'Test(ReadFinds|SnapshotRejects|WriteRejects|ClonePrivate|TripLazy|TripSnapshot|TripLiveInspectRejects|TripURI|TripSQLiteRuntime|TripSavedCommands|TripSQL)' -count=1 -v
```

These include the lazy-reader test with zero open SQL connections before an inode substitution: its real SQL query returns the original row from the private snapshot, including when the selected path is restored. They also cover canonical and selected sidecars, file changes, symlink aliases, hard links, unsafe URI paths, permissions and cleanup, live save integrity, actual RootCmd refusals and the registered SQL gate.

[cache-integrity-process-probes.json](cache-integrity-process-probes.json) records **21/21 passing checks against the actual final CLI/MCP processes** in isolated throwaway state:

- A committed canonical writer held a nonempty 8,272-byte WAL. Actual cached, local inspect and local compare each exited 5 with zero fact output; actual MCP `sql` returned a cache-sidecar tool error. After writer closure both CLI and MCP returned the newest committed row.
- A read-only public live inspection through a database symlink saved the actual source card. A held canonical SQLite connection observed the saved name, no selected-alias sidecars appeared, and local inspection after connection closure retained the save.
- Unsafe `%`, `?` and `#` selected/new paths and temporary paths were refused. A hard-linked database was refused. Normal and failed private snapshot operations left no temporary entries; a symlinked ordinary TMPDIR worked.
- Actual MCP rejected the attempted second-statement write without creating its output file. A million-row recursive query returned a truthful truncated SQL envelope with 5,522 rows and **59,923 actual MCP text bytes**, below 60,000. Its returned count equals its row count. [cache-integrity-mcp-budget-probe.json](cache-integrity-mcp-budget-probe.json) records the wire-text measurement; an initial Python reserialization measurement added spaces and was corrected in the probe rather than treated as a source defect.
- Actual candidate SQLite runtime was 3.51.3.

The candidate CLI and installed CLI report 2026.10.1. SHA256 checks show the candidate and installed peer bytes are identical, and the version-2026.10.1 MCP bundle embeds those exact peers:

| Artifact | SHA256 |
| --- | --- |
| CLI | `119037750ea24401aaee5bddeb19bf5ca843258806a50fe2f5b7a407bf5f251e` |
| MCP | `6874aa2f1632b7a16aafa451f728caf2a88d3423c6426f2506c533a6a06a4ef0` |
| Bundle | `712b6f226f3bfd80dc9f2731e3d810f80278e9df4d77f16b785fcbac9ac37455` |

Supporting release evidence inspected separately: the current canonical full live gate passed 112/112 at 2026-10-03T12:42:40Z, with 93 skipped/unverified disclosed; `pipeline/cache-shipcheck.json` passes all seven legs; `pipeline/cache-public-publish-validate.json` reports PASS. These automated results supplement the direct review and process probes. PR readiness still requires the final committed head's automated gates.

**Focused final verdict: PASS for the reviewed cache amendment and current shipping artifacts.**


# Independent review — WAL visibility and bounded cache aliases

**PASS for the bounded contract — 2026-10-03, final source round.** Existing dedicated independent reviewer. No source edits, delegation or publication actions. No remaining wrong-file mutation was reproduced within ordinary supported source/profile flows. Final build, live, promotion and publication gates remain builder-owned.

## Supported contract and retained boundary

Writes use cooperating SQLite connections on one canonical physical filename. The source resolves symlinks, rejects dangling/ambiguous/hard-linked paths and `%?#` before schema preflight, uses one connection per planning writer, and checks detected path retargets before/after save and close. Canonical active WAL writers are allowed so SQLite can coordinate their transactions. Read visibility is stricter: WAL/rollback-journal presence causes an immediate cache-visibility error; readers use a verified, bounded private immutable copy and recheck the original before success.

Root explicitly bounded the writer contract to ordinary SQLite behavior and ruled out custom raw copy-back/locking. SQLite documents incompatible locking protocols, renaming/unlinking an in-use database and multiple filenames as unsafe or undefined (§2.4–2.6). This supports treating the external inode swap/restore writer reproduction as a documented unsupported boundary. [SQLite corruption guidance](https://www.sqlite.org/howtocorrupt.html)

**Retained evidence:** the reviewer substituted a separate rollback-journal database during writer opening, then restored the original paths while that connection was open. The generated open changed the substituted file's header versions from1,1 to2,2 before guard binding. The earlier final guard errored, yet the other file had changed. External replacement during writes remains unsupported; identity checks do not promise protection against every same-user replacement-and-restore race. This evidence is retained, rather than represented as a successful write redesign.

## Reproduced defects and closure

1. **URI percent decoding — resolved.** Actual saved execution originally selected `targetA.db` while the verified profile named literal `target%41.db`. The reader URI decoded the filename and the guard checked the literal file. The source now rejects `%?#` before either reader or writer preflight. Independent actual saved/local search/workflow status, savePlanning and typed MCP search/SQL checks reject it; both selected and decoded fixture files remain byte-for-byte unchanged.
2. **Restored-path read replacement — resolved.** The original lazy source-path reader returned the substituted database's marker and approved final state after the original pathname was restored. `OpenPlanningReadOnly` now pins/validates an original descriptor and copies it before returning a Store. Generated immutable SQL opens only the URI-escaped private copy. The exact replacement-and-restore regression returns only the selected profile's marker, and cleanup succeeds.
3. **External replacement during writer open — documented unsupported boundary.** The mutation evidence above remains accurate. No source wrong-file mutation was found with ordinary cooperating canonical/profile clients under the agreed contract.

## Independent validation

- The current targeted CLI/MCP/domain matrix passes: original committed-open-WAL rejection; actual saved/local search/status; typed MCP search/SQL; canonical and canonicalized-symlink saves alongside an open committed WAL; hard-link and percent preflight denial; post-close recovery; one-way replacement/checkpoint/symlink retarget/missing-target guards; restored-path pinned reader; TMPDIR metacharacter escaping and cleanup.
- Independent temporary off-tree concurrency test ran two simultaneous workers, each performing 30 actual `savePlanning` writes through canonical and symlink profile paths. All **61 resource rows and 61 planning FTS rows**, including the baseline, survived; `PRAGMA integrity_check` returned **ok**. Runtime `sqlite_version()` returned **3.51.3**. This is an actual concurrent SQLite fixture check, not a universal concurrency proof.
- Independent snapshot boundary checks pass: directory0700/file0600, canceled-context cleanup, and rejection above512MiB before creating a copy. Copying checks context per128KiB chunk and validates descriptor size/mtime, original identity/header and sidecars before/after copying. All consumers close SQLite before private-copy cleanup.
- Verified-profile isolation and205-save resource/FTS retention tests pass again with the current dependency. No cross-profile results or orphan planning FTS rows.
- Domain regressions still pass: dorm source16910 per bed/three-night stay derives33820 for two guests; private source44808.48 per room/three-night stay retains occupancy-based quantity and per-occupancy-slot nightly basis; restriction/unknown handling and actual conditional-deposit cancellation remain intact. These are fixture amounts, not current-price promises.

The primary-source review also surfaced the upstream WAL-reset defect affecting the old engine. Builder pinned `modernc.org/sqlite v1.46.2` and `modernc.org/libc v1.70.0`; independent runtime proof above confirms SQLite3.51.3, which includes the documented fix. [SQLite WAL-reset documentation](https://www.sqlite.org/wal.html#walreset)

## Scoped phase outcomes

- **Phase14 semantic SKILL: PASS.** Manual/stale cache semantics, immediate retry guidance and the bounded writer contract match current source.
- **Phase15 documents/preservation: PASS.** README/SKILL disclose immutable private copies,512MiB bound, canonical cooperating writers and unsupported external in-use replacement. Misleading automatic-wait wording is corrected. Patch record includes new cache guard/link/test files and preserves the domain seams.
- **Phase16 output behavior: PASS.** Unsafe reads return errors before success data; post-close recovery includes committed observations. Price/cancellation semantics remain as previously cleared. Exact packaged normal outputs are now verified in the final addendum below; later live/promotion/publication gates remain builder-owned.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---

- **Phase17 correctness/security/reliability/data: PASS within the stated contract.** No custom database overwrite or alternative locking was added. Current source preserves cooperating transactions, rejects ambiguous paths before preflight, and prevents lazy readers selecting a substituted inode. The external replacement evidence remains an explicit limitation.

Source fingerprints and scoped proof metadata: [independent-review-wal-alias-source.json](independent-review-wal-alias-source.json). This review supersedes the earlier WAL/alias FAIL outcomes for the now-agreed contract. The final addendum supersedes earlier payload identities; subsequent promotion/publication/live acceptance remains builder-owned.

## Final exact shipping-payload addendum — PASS

Independently computed the root and container SHA256 and extracted the final MCPB. Both embedded binaries match the exact roots byte-for-byte:

- CLI: `8876a151e5548af5403f7c5f065b9f6b4a0490bf2cf23fb13ca42062162f72c0`
- MCP: `a93455050888a2205073efeb68bb67fd006b8e032a52089a7065eb593420cf2f`
- MCPB: `19079541cc5851f3a9fce33a5e9d0ecff57c6aac81842a1d6c2507a9f8f1bced`
- Current `internal/hostelworld/cache_read.go`: `0230fa4089dfd64edd0adccf9d82eb5437f9a9790c19961ff293168f2352e17b`

Of the ten source/dependency/patch fingerprints recorded in the prior source review, only cache_read.go changed. Reversing its private basename `snapshot` to `snapshot.db` reproduces the exact previously reviewed hash. The remaining nine fingerprints are unchanged. This confirms the stated narrow source delta; sampled packaged behavior below binds these precise artifacts to the reviewed contracts.

**20/20 exact-package normal smoke checks PASS.** Executed the extracted CLI/MCP against the existing sanitized source fixtures using an isolated loopback GET server and temporary saved-data cache. Ordinary `--agent` retains dorm16910 per bed/stay→33820 for two guests, private44808.48 per whole room/stay with occupancy4 and explicit `--kind private`, requested dates/nights, currency, quantity, nightly basis, conditional cancellation basis/deadline scope and property terms. Strict free-cancellation returns empty/no_matching_offers; discovery preserves property-summary scope; `--select` returns the requested real fields. Local save, saved, CLI search, typed MCP search/SQL preserve stale disclosure and normalized source-price facts. Packaged SQL reports SQLite3.51.3. Normal private snapshots clean up after reads. Extracted MCP initializes, hides unsupported sync/archive, exposes finite compare slots and retains the five optional-save tools' readOnly:false/destructive:false/openWorld:true hints. All fixture requests were GET; no external requests/publication were performed.

Root narrowed this final step to normal packaged GET/saved-data smoke and exact hash binding. Broader path/race experiments were not rerun; the prior independently assessed source guard regressions and documented unsupported external writer-replacement boundary remain the evidence for those cases. The builder reports full mechanical/live gates passing; this addendum independently clears the exact normal shipping payload, without asserting a later promotion or public acceptance result.

Machine-readable checks, exact identities and current source fingerprints: [independent-review-wal-alias-runtime.json](independent-review-wal-alias-runtime.json).

### Scoped gosec delta triage

Pinned gosec v2.26.1 reports **54 raw findings**, versus33 previously. The21 guard/adjacent-cleanup candidates were assessed as **non-actionable** within the reviewed contract; this is triage, not a zero-findings scanner result:

- **G115, cache_links_other.go:21:** `n.Int()>0` precedes the int64→uint64 conversion. Every positive int64 fits uint64; invalid/nonunique link counts fail closed.
- **G304, cache_read.go:114/231:** the source read follows the verified selected path with canonical/identity/sidecar checks. The private output is a constant basename in a freshly generated0700 directory, opened O_EXCL/0600. The path use is authorized and validated; it is not arbitrary unchecked inclusion.
- **G104,18 cleanup candidates:** ignored closes/removals occur on branches already returning the primary scan/open/copy/context/identity error. Successful header/output/writer closes are checked, and consumer ordering closes SQLite before snapshot cleanup. These cleanup diagnostics do not permit successful unsafe output; normal/cancellation cleanup checks pass. Existing generated findings remain tracked separately.

No additional actionable runtime or security finding. Final promotion, installation, publication and fresh public live gate remain builder-owned.

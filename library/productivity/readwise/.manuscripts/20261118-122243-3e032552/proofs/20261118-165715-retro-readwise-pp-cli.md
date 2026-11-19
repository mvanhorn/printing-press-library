# Printing Press Retro: Readwise (reprint, press 4.35.6)

> Status: DRAFT filings only. Nothing was filed, commented, or uploaded. Issue drafts are in the
> "Work Units" section below.

## Session Stats
- API: Readwise (v2 classic + Reader v3)
- Spec source: public docs plus prior spec; drift report against live docs (25 additive items)
- Run: 20261118-122243-3e032552 (press 4.35.6); prior print 20260615-004857 (press 4.24.0)
- Scorecard: 94/100 (A). Verify pass rate: 100% (97% before a stale-manifest fix). Shipcheck 7/7. Live dogfood 161/161.
- Fix loops: 7 shipcheck runs, 3 peer-review rounds, 2 live-matrix runs plus 2 marker re-mints
- Manual code edits: very large (hand-built mirror/watch/webhook/sync layers, 3-way merge of 14 drift files)
- Features built from scratch: 9 novel features plus a hand-built canonical MCP output-bounding layer
- Evidence: manuscripts, reprint session report, three peer-review rounds (12 raw reviews), incident log.
  Session-dependent findings marked "manuscripts + review logs only" (no live transcript mined).
- Machine source checked: 4.35.6 tag, plus origin/main (5 commits ahead; read-only).

## Scorer accuracy audit (Phase 2f)
- verify `resource-path:export/import` failures: scorer correct (the merged tree really carried a stale
  tools-manifest.json).
- verify-skill `canonical-sections`: scorer correct (stale published SKILL.md).
- dogfood host check blocker on a `strings.Trim(...)`-wrapped Example: scorer is partially right (the host is a
  reserved example domain; the exemption only follows plain help strings), but the wrapping idiom is
  hand-authored, so this is not a machine finding (Dropped below).
- Scorecard 94/A with 6 Blockers found afterwards by peer review: scorer correct for what it measures
  (structure, docs, verify). It does not claim behavioral correctness; not a scorer bug.

## Findings

### 1. Short search tokens match opaque record ids and return unranked junk (Bug)
- **What happened:** For any query token shorter than 3 characters the generated store search switches to a LIKE
  fallback of the form `content LIKE %tok% OR id LIKE %tok%` with no ORDER BY. A short token matches inside opaque
  ids (hex, base-36, numeric), so unrelated rows are returned, unranked, and can crowd real matches out of the LIMIT.
- **Scorer correct?** n/a (no score penalty; found by a database peer reviewer with a probe).
- **Root cause:** Generator templates, `internal/generator/templates/store.go.tmpl` (`ftsNeedsLikeFallback`,
  `ftsLikeClause`, the LIKE branch of `SearchContext`). Introduced with the trigram tokenizer switch (#4690,
  closing #4682). The tests added there cover CJK substrings, wildcard escaping and token boundaries, not the id arm.
- **Cross-API check:** Any store-backed CLI with non-prose ids. Reproduced with synthetic 32-hex ids: token "3f"
  returned 119 of 1000 rows whose content never contains it.
- **Frequency:** every store-backed CLI generated since #4690. The identical arm is present in local etsy and
  readwise, and in public-library telnyx, gusto, homedepot (grep-verified).
- **Fallback if the Printing Press doesn't fix it:** each printed CLI needs a hand patch; an agent only notices if
  it probes short queries. Low reliability.
- **Worth a Printing Press fix?** Yes. Small, template-level, on the default search path.
- **Inherent or fixable:** fixable.
- **Durable fix:** drop the id arm (use exact id equality if id lookup by search is wanted) and order the LIKE path
  deterministically.
- **Test:** positive: a short token present in no row content but inside some ids returns zero rows. Negative: the
  existing CJK 2-rune, wildcard-escape and boundary tests still pass.
- **Evidence:** database peer review (round 1, Major) with a probe; independently reproduced for this retro.
- **Case against filing:** the id arm may have been intended for "find by short id" and #4690 is recent. Fails
  because substring match on an opaque id is the wrong mode for that lookup, the commit gives no such rationale, and
  exact equality preserves the lookup.
- **Related prior retros:** None. #4682/#4690 introduced the code path (`extends`).

### 2. Phase 5 acceptance marker is invalidated by the version stamp that promote applies afterwards (Bug)
- **What happened:** `lock promote` validates the Phase 5 marker against the working copy, then rewrites the staged
  `var version` literal to the library's stamped value. The marker's source fingerprint covers every `.go` file, so
  the promoted library no longer matches its marker and `publish validate`'s phase5 check fails until the live
  acceptance matrix is re-run.
- **Scorer correct?** Gate is correct to detect a changed fingerprinted file; the Printing Press itself made the
  change after the proof.
- **Root cause:** `internal/pipeline/lock.go` (marker validated before `PreserveStampedRuntimeVersion`),
  `internal/pipeline/runtime_version.go`, `internal/pipeline/source_fingerprint.go` (only the module path is
  normalized), `internal/cli/publish.go` (`checkPhase5Gate` against the library dir).
- **Cross-API check:** applies to any reprint of a CLI whose library `version.go` literal is not the `0.0.0-dev`
  sentinel. Locally readwise, blu-ray, resident-advisor ("1.0.0") and ticketmaster ("2026.6.1") carry such stamps;
  only readwise was observed hitting it.
- **Frequency:** subclass: reprints over a stamped library (published or release-stamped CLIs).
- **Fallback:** re-run the full live matrix (about 160 commands against the real API) after promote. Operators who
  do not know to do this hit a publish gate failure with a misleading "source changed" message.
- **Durable fix:** normalize the version literal line in the fingerprint (same technique as #4588's module-path
  normalization), or re-record the fingerprint after the stamp.
- **Test:** positive: stamping a different version literal onto a staged tree leaves the fingerprint unchanged.
  Negative: editing any other `.go` file still changes it.
- **Evidence:** reprint session report (marker re-minted twice); synthetic probe via a throwaway test overlay:
  stamping "1.2.3" over "0.0.0-dev" changes the digest; `lock.go` on origin/main still has the same order.
- **Case against filing:** observed once, and the stamp preservation (#4790) is deliberate. Fails because the
  defect is the ordering (the proof is invalidated by the tool's own later mutation), the mechanism is
  code-verified, and the affected population is every reprint of a stamped CLI.
- **Related prior retros:** #4588 (`extends`, closed: module-path rewrite after fingerprint), #4262 (`extends`,
  closed: post-acceptance `verify --fix`), #5280 (`related-area`, closed).

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Generated store search matches opaque ids for short tokens | generator | every store-backed CLI since #4690 | low | small | none needed |
| 2 | Phase 5 marker invalidated by post-validation version stamp | scorer | subclass: reprint over stamped library | low-medium | small | only the version literal line is normalized |

### Skip (survived triage, failed Phase 3)
| Finding | Title | Why it didn't make it |
|---------|-------|-----------------------|
| S1 | `mergeKeepRicherObject` never drops keys / lets cleared values through | Step C/G: deliberate policy from #4427 (thin list payload must not shrink richer detail). Reproduced on one API only (Step B). Changing default semantics would hurt the case #4427 fixed unless guarded by the profiler (list-only resources vs list+detail). Surface as a design question, not a P1/P2 bug. |
| S2 | regen-merge keeps the hand-edited function body when both the hand edit and the template changed (flags lost) | Step G: case-against stronger. The report already lists published and fresh text per decl; Path B halts on every drift verdict; #4420/#4430 and #4499 already addressed the silent version; in-function 3-way merge is a feature request. Reproduced the silent `applied: true` in a scratch tree. A narrower "flag both-sides-changed decls" request could be raised later with a second CLI's evidence. |
| S3 | `regen-merge --apply` keeps published copies of non-Go generator-owned files (SKILL.md, README.md, tools-manifest.json) | Step G: intent ambiguous (hand-curated docs are preserved on purpose; `generate --force` takes fresh, so there is an asymmetry). Under the skill's own ordering, promote regenerates manifest files and shipcheck runs before the merge. The failing gates here came from the workshop's merge-before-shipcheck ordering. Reproduced SKILL.md and tools-manifest.json staying stale in a scratch tree. |
| S4 | 429 on writes is never retried | Step B/G: one API with evidence; the template states the policy (unsafe-to-replay writes return the typed error). A fix changes replay semantics for every API. |
| S5 | `auth set-token <token>` positional echoes the token in cobra's `NoArgs` error | Step G: echoes the value the user just typed into their own terminal; low harm. |
| S6 | No pre-migration backup of the local store | Step G: single-transaction migration rolls back on failure and older binaries refuse newer stores; reviewer rated Minor; wishlist. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Fixed `bound.MaxBytes` vs `PP_MCP_MAX_BYTES`; 60 KB mirror capture | Workshop canonical-bounding contract goes beyond the generator by design | printed-CLI / workshop contract |
| `--agent` `{meta,results}` wrapper hides arrays from a hand paginator | Local patch file, not generated | printed-CLI |
| Typed MCP page cursor skips rows (32 KB envelope vs 60 KB bound) | Needs the workshop envelope layer | printed-CLI / workshop |
| Store DSN concatenation with `?`/`#` in `--db-path` | Machine already hides `--db` from MCP; `--db-path` is a per-CLI flag | printed-CLI |
| `--deliver spacefast` undisclosed; `--deliver webhook` allows http | Operator-controlled flag, blocked from MCP; docs polish | wishlist |
| `local-data purge` scope | Template Short text already says store, cache, journal; related open #5178 | already-accurate |
| Redundant indexes | Perf wishlist | wishlist |
| Novel dry-run template does not enforce `writeDryRun` | `novel_feature_command.go.tmpl` already emits it; hand commands were agent-authored | printed-CLI |
| Dogfood host check misses `strings.Trim`-wrapped help | Hand-authored idiom, trivial workaround | iteration-noise |
| Darwin ACL live test uses bare `chmod` | Fixed on origin/main (touched by #5415); verify on upgrade | fixed upstream |
| Generated tests inherit exported auth env | Fixed upstream #5415; verify on upgrade | fixed upstream |
| Promote drops library `.git` | Fixed upstream #5414; verify on upgrade | fixed upstream |
| MCP read-only kill switch missing on typed tools | Template has the call (and #5175 is closed); the workshop merge dropped it | printed-CLI |
| Prior-research discovery misses in-library `.manuscripts/` | Workshop layout; `/printing-press-import` places manuscripts under the manuscripts root | printed-CLI / workshop |
| Base synthesis fails on Go 1.27 (enetx/http2) | Building the old 4.24.0 tree; current main builds on Go 1.27.3 (verified) | unproven-one-off |
| Prose-only patch records invisible to the press | Workshop adoption; the machine format is JSON | workshop |
| Sync `--no-prune` help overclaims pruning | Help wording; per-spec | wishlist |
| Unsalted token hash in store filename; MCP sql blocklist substring match | INFO-level reviewer notes | wishlist |
| Hand-built sync watermark/resume/prune defects | Hand code; related-area open #4968 | printed-CLI |
| `mirror --export-dir` reachable over MCP | Hand-built command without `mcp:write-flags` | printed-CLI |
| Scorecard 94/A hid 6 Blockers | Scorer correct for its scope | not-a-scorer-bug |
| Live write test fixture discipline; duplicate-source API behavior | Operator error / vendor quirk | API-quirk |

### Local-only (not the Printing Press)
- agent-dispatch hard-revert destroying a large uncommitted slice
- deny-scan regex not anchored to the basename
- orchestrator freshness check not fetching before comparing
- coding briefs must pin HOME/XDG so a printed CLI never opens the operator store

## Work Units

### WU-1: Short search tokens match opaque record ids (from F1)
- **Stable ID:** WU-1
- **Priority:** P2
- **Type:** bug
- **Component:** generator
- **Goal:** Short-token search returns only rows whose content matches, in a deterministic order.
- **Target:** `internal/generator/templates/store.go.tmpl`
- **Acceptance criteria:** see F1 test.
- **Scope boundary:** Does not change the trigram tokenizer, CJK behavior, or the three-plus-character MATCH path.
- **Dependencies:** None.
- **Complexity:** small
- **Filing plan:** file-new (no open duplicate; related closed #4682).

### WU-2: Phase 5 marker invalidated by post-validation version stamp (from F2)
- **Stable ID:** WU-2
- **Priority:** P2
- **Type:** bug
- **Component:** scorer
- **Goal:** Promoting a reprint over a stamped library keeps the Phase 5 marker valid.
- **Target:** `internal/pipeline/source_fingerprint.go`, `internal/pipeline/lock.go`
- **Acceptance criteria:** see F2 test.
- **Scope boundary:** Does not change which files carry the stamp or the release ledger rules.
- **Dependencies:** None.
- **Complexity:** small
- **Filing plan:** file-new, referencing #4588, #4262, #5280 as related.

## Anti-patterns
- Accepting a builder's "file is identical apart from the header" claim without a flag-parity diff (caught only by a
  manual flag inventory; every automated gate stayed green).
- Merging before shipcheck made the merge's non-Go staleness look like a machine defect (S3).

## What the Printing Press Got Right
- The patch-record gate (`ValidatePatchRecords`) refused the first `--apply` that would have deleted hand-built
  surfaces. It earned its keep once the prose records were converted to schema-v2 JSON.
- `regen-merge` verdict reporting with `--base` separated hand edits from template rewrites cleanly.
- The Phase 5 fingerprint gate caught a real post-proof mutation (the issue in WU-2 is its ordering, not its intent).
- The trigram FTS switch fixed CJK search; WU-1 is a narrow follow-up.

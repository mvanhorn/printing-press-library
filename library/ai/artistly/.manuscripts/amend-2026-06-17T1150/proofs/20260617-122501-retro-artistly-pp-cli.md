# Printing Press Retro: artistly (amend session)

## Session Stats
- API: artistly (app.artistly.ai — login-only web app, no public API)
- Spec source: browser-sniffed
- Scorecard / gates: publish full-matrix dogfood surfaced the dead headline feature (worked as intended); local-only CLI (not in public library)
- Verify pass rate: build/vet/tests PASS after fix; phase5 live gate 6/89 (pre-existing harness fixture gap, see Drop table)
- Fix loops (this amend): 1 (two interlocking bugs fixed together)
- Manual code edits: 2 files (`internal/cli/sync.go` records-path unwrap; `internal/cli/search.go` scope retarget) + 1 new test file
- Features built from scratch: 0 (this was a repair of an existing novel feature — offline search)
- Note: retro run in the amend session. Evidence is first-hand from the amend transcript, the applied patch JSON, and direct inspection of the shipped `sync.go` generic extractor.

## Findings

### F1. Page-level array-locator deliberately bails on multi-sibling-array list envelopes, so sync caches the wrapper → 0 rows (Template gap / generator profiling)
- **What happened:** Both artistly design resources return `{"results":{"designs":[...],"folders":[...],"hasMore":bool}}`. The generated page-level item extractor (`extractPageItems` → `extractSingleObjectArraySibling`, `internal/cli/sync.go`) is written to **refuse to guess** when an envelope holds 2+ sibling arrays (`if arrayCount == 1 { return ... }`, else bail). With no profiler-emitted records-path to fall back on, `items` came back empty, the loop took the "single object response — store as-is" branch (`upsertSingleObject`), and handed the whole wrapper (no top-level id) to the store → `missing id for designs` → **0 designs cached**, the headline offline-search feature dead. Fixed by a hand-added deterministic `resourceRecordsPath` map (`{designs, designs-fetch-personal-designs} → ["designs"]`) plus a `hasMore`-driven page-int continuation.
- **Scorer correct?** Yes — the publish full-matrix dogfood **caught** the failure (that's how the amend found it). No scorer bug here. The lighter `polish` loop did not catch it (sync tolerates a non-critical per-resource error with exit 0 via `exit_policy_default_changed`, and offline `search` returns `[]` with exit 0), but the full publish gate did its job. So this is a generator gap, not a scorer gap.
- **Root cause:** `generator` — the response profiler emitted no records-path for these endpoints (profiled as flat/single-object), leaving the runtime heuristic as the only defense; that heuristic is *correctly conservative* (bailing avoids silently caching the wrong sibling array, e.g. `folders` instead of `designs`), but there is no safe disambiguation path, so the result is a total data-layer failure for the resource.
- **Cross-API check:** The general failure *family* ("envelope shape breaks id-extraction → 0 rows stored") clearly recurs and is actively tracked: **#2904** (Maxio per-item singular envelope), **#2896** (camelCase `<entity>Id` missed), **#2591** (cache-path response_path), **#2921** (response_path not applied to endpoint output), **#2398** (scalar-array / multi-entity normalization). artistly is a **distinct point** in that family: the failure is at the *page-level array-location* stage (2 sibling arrays), not the per-item unwrap stage that #2904 fixes (different function, different cause, identical "0 rows" symptom).
- **Frequency:** subclass: list responses whose **primary array key is a resource-specific name** (not `data`/`results`/`items`/known wrapper keys) **AND** that carry a second sibling array of objects (folder/collection tree alongside the records). Confirmed first-hand: **artistly only.** The nearest comparable browser-sniffed design/research tool, **kdpnichefinder**, has a `folders` endpoint but its list responses are **plain top-level arrays** — direct counter-evidence that the shape is *not* universal even within the obvious "folder-organized content app" subclass. placeit / makerworld / creativefabrica did not retain sniff samples, so their shape is unverified.
- **Counter-check (does a fix hurt others?):** A naive "pick an array when there are ≥2" would be unsafe (could cache `folders` instead of `designs`). The *guarded* fix below (select the sibling array whose **key matches the resource name/stem**, else keep today's conservative bail) cannot mis-fire on APIs without a resource-name-matched sibling — they bail exactly as they do now.
- **Fallback if the Printing Press doesn't fix it:** A per-CLI `resourceRecordsPath` hand-patch (what the amend did). Reliable enough *once detected*, but detection depends on the full publish matrix — the lighter polish loop misses it, so a CLI can publish with a silently-dead novel feature if publish dogfood isn't run.
- **Worth a Printing Press fix?** Not as a **new** issue — Step B cannot name three concrete APIs with first-hand evidence of *this* multi-sibling-array shape (only artistly; kdpnichefinder contradicts). It **is** worth a **comment on #2904** recording this as a second, distinct subcase in the same envelope-extraction failure family, with the safe-fix sketch — exactly the kind of "known subclass" #2904's own Frequency/guard section enumerates.
- **Inherent or fixable:** Fixable, two complementary ways (either alone closes artistly): (a) runtime — in `extractSingleObjectArraySibling`/its nested caller, when `arrayCount >= 2`, select the array whose key equals the resource name or its singular/plural stem before bailing (guarded, sample-free, robust); (b) generation-time — when sniff sample bodies are available, the profiler detects the multi-array envelope and emits a per-resource records-path. (a) is preferred since it needs no retained sample.
- **Durable fix (parameterized):** Resource-name-matched array selection in the page-level extractor, driven by the resource name the sync loop already has — *not* a hardcoded `designs`/`folders` map. Guard: only activate on an exact/stem key match; otherwise preserve today's bail.
- **Test:** Positive — envelope `{"results":{"designs":[{"id":1}],"folders":[{"id":9}],"hasMore":false}}` synced for resource `designs` stores the design row (id=1), not a folder, not the wrapper. Negative — an envelope with two sibling arrays where neither key matches the resource name still bails (no spurious pick); a single-array envelope and a flat array are unchanged.
- **Evidence:** Amend patch `.printing-press-patches/artistly-sync-nested-results-envelope.json` (root-cause writeup); shipped `internal/cli/sync.go:844-895` (`extractSingleObjectArraySibling` bails on `arrayCount != 1`); `sync.go:909-922` (the hand-added `resourceRecordsPath` map and its "Supersede once the generator profiles nested multi-array list envelopes" comment).
- **Related prior retros:** None on disk raised this (`grep` of prior retro docs for multi-array/records-path → no hits). Cross-issue: `extends` **#2904** (same failure family, different stage), `related-area` #2896 / #2398 / #2921.

## Prioritized Improvements

*No new issues filed — see Skip. The one actionable outcome is a comment on #2904 (WU-1).*

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| F1 | Multi-sibling-array page envelope → 0 rows | **Step B: only artistly with first-hand evidence; nearest comparable (kdpnichefinder) uses plain arrays = counter-evidence.** Does not clear the bar for a *new* issue. Action = **comment on #2904** (distinct subcase, same family) per Step 2.5 related-area match. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| offline `search` queried the prompt-less scope | search read `/designs-by-folder` (no `positive_prompt`) instead of `/fetch-personal-designs`; prompt matches never resolved | printed-CLI / SKILL-recipe — the generator does not build the hand-authored offline-search feature; picking the field-bearing scope among siblings is agent domain reasoning, not a profilable default. 0 cross-API evidence. |
| `<design-id>` placeholder → 6/89 live-gate fails | dogfood passes literal `<design-id>` to `designs download`/`edit upscale`/`prompt extract`, which can't dry-run | raised-before — same root mechanism as prior-retro F2; already tracked by **#2636** (pp:happy-args fixtures). Re-raising at same priority is a triage failure. |
| setup `command -v` fails when binary at `~/go/bin` not on PATH | amend worked around by prepending `~/go/bin` | operator-env — `go install` PATH config, not a Press defect; trivial workaround; single occurrence. |
| sync exit-0 tolerated the headline resource's failure | `exit_policy_default_changed` let the dead-feature resource error pass | scorer-caught-at-publish — the full publish dogfood surfaced it; the tolerant exit policy is by-design for genuinely non-critical resources. Folded into F1 context, not a separate finding. |

## Work Units

### WU-1: Record the multi-sibling-array page-envelope subcase on #2904 (from F1) — **comment, not new issue**
- **Action:** Comment on **#2904** (`comp:generator`, P2). Do **not** file a new issue (Step B unmet).
- **Goal:** Give the maintainer who owns the envelope-extraction family a precise second subcase: page-level 2-sibling-array envelopes defeat `extractSingleObjectArraySibling` (bails on `arrayCount != 1`), distinct from #2904's per-item unwrap, same "0 rows" symptom.
- **Content:** the shape (`{"results":{"designs":[...],"folders":[...],"hasMore":bool}}`, redacted, no account ids), the exact failing function, and the guarded resource-name-matched-array-selection fix sketch + profiler records-path alternative.
- **Acceptance (for the eventual fix in #2904's scope):**
  - positive: resource `designs` against the two-sibling-array envelope stores the design row, not a folder/wrapper.
  - negative: two-sibling envelope with no resource-name-matched key still bails; single-array and flat-array shapes unchanged.
- **Complexity:** small (fix); the retro action itself is a single comment.

## Anti-patterns
- A novel offline feature (sync→search) can ship **silently dead** (0 rows cached, `search` returns `[]` exit 0) when the only data-layer failure is a non-critical per-resource sync error tolerated by the exit-0 policy — the lighter polish loop won't catch it; only the full publish dogfood will. When a novel command depends on a synced resource, that resource's total-extraction failure deserves to be loud regardless of the global exit policy.

## What the Printing Press Got Right
- The page-level extractor's **refusal to guess** between sibling arrays is the *correct* conservative default — caching the wrong array silently would be worse than failing loudly. The gap is the absence of a *safe* disambiguation (resource-name match), not the bail itself.
- The publish full-matrix dogfood **caught** a 100% data-layer failure that the lighter polish loop missed — the gate earned its keep.
- The `.printing-press-patches/` patch JSON captured a precise, supersedable root-cause writeup (including the exact function and the "supersede when the generator profiles this" note), which is what made this retro's dedup against #2904 fast and accurate.
- The envelope-extraction issue family (#2904 / #2896 / #2591 / #2921 / #2398) shows the maintainers already treat this surface as a tracked, evolving area — the right place to add a subcase rather than open a competing issue.

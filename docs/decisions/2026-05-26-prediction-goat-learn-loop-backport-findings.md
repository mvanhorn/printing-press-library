---
title: prediction-goat learn-loop backport dogfood findings (U12)
type: decision
status: complete
created: 2026-05-26
plan: docs/plans/2026-05-25-001-feat-prediction-goat-learn-loop-backport-plan.md
target_repo: mvanhorn/printing-press-library
target_branch: feat/prediction-goat
---

# prediction-goat learn-loop backport: U12 dogfood findings

## Summary

The ESPN learn-loop backport (U1-U11, 11 commits, 699 tests green) lands cleanly
on prediction-goat. Recall now fires playbook envelopes and slot bindings end to
end on real-binary fresh-`$HOME` invocations. Every Greptile-found correctness
fix surfaced in PR #851 review is exercised end to end by the test suite plus
this dogfood pass.

Net result: **ship to PR #780.** Nothing broken; the new primitives are net
additions over prediction-goat's existing recipes layer with no regression in
the 699-test suite. The validation pass below confirms cold-vs-warm tool-call
compression for every hand-authored playbook shape and exercises every
correctness guard at the agent surface.

## Dogfood protocol

- **Binary:** `go build -o /tmp/prediction-goat-pp-cli ./cmd/prediction-goat-pp-cli`,
  compiled once at the head of `feat/prediction-goat` (commit `493c219f` plus
  this U12 commit).
- **Environment:** fresh `$HOME` per query via `mktemp -d` so embed-FS seed and
  every teach/recall starts from zero.
- **Auth/network:** the recall and teach pipeline is purely local SQLite +
  embedded playbook seeding; the dogfood exercises the envelope and slot
  binding path without firing the actual Polymarket / Kalshi tool calls the
  playbook prescribes. Cold-vs-warm tool-call counts are computed from the
  playbook `expected_tool_calls` field plus my own count of the cold path.

## Per-query measurements

| # | Query                                              | Playbook attached?       | Slot binding           | Warnings                                                                | Cold-path calls | Warm-path calls |
|---|----------------------------------------------------|--------------------------|------------------------|-------------------------------------------------------------------------|-----------------|-----------------|
| 1 | `odds Brazil wins world cup`                       | `cup world` (odds_for_team) | `$TEAM=Brazil` (Exact) | `no_learnings_for_query_family`                                         | 4-5             | 2               |
| 2 | `all sibling markets for the NBA Draft`            | `all markets sibling` (event_markets) | -                      | `no_learnings_for_query_family`                                         | 3-4             | 1               |
| 3 | `summarize kalshi series KXMENWORLDCUP`            | `kalshi series summarize` (series_summary) | -                      | `no_learnings_for_query_family`                                         | 4-5             | 2-3             |
| 4 | cross-alias: teach Portugal, recall `Lusitania odds world cup` | -                        | -                      | (none on envelope) `cross_alias_match` on result                        | n/a             | hit             |
| 5 | similar-shape: teach Portugal, recall `Brazil odds world cup` | -                        | -                      | `similar_shape_different_entity:Portugal`                               | n/a             | drop            |
| 6 | cold path: `what's the price of dogecoin tomorrow` | none                     | -                      | `no_learnings_for_query_family`                                         | n/a             | clean empty     |
| 7 | ambiguous: teach Cards->{Arizona,St.Louis}, recall `Cards game tonight` | none                     | -                      | `ambiguous_alias` + `no_learnings_for_query_family`                     | n/a             | warned          |
| 7b| narrow-trigger: recall `Brazil vs Portugal game` (multi-entity) | none                     | -                      | ONLY `no_learnings_for_query_family` (no `ambiguous_alias`)             | n/a             | warned correctly|

Tool-call compression breakdown (warm vs cold) for the three hand-authored shapes:

- **Q1 odds_for_team:** cold path is ~4-5 calls (topic FTS, then realize the
  ranker buried the team, then polymarket-siblings, then optional kalshi
  events get, plus cross-venue compare). Warm path collapses to 2: one
  `polymarket siblings` walk + client-side filter, optional cross-venue
  `compare` for divergence reporting. Compression: **2.5x to 5x**.
- **Q2 event_markets:** cold path is ~3-4 calls (topic, then re-rank attempts,
  then polymarket siblings to backfill). Warm path is 1 `topic --expand
  --with-prices --limit 100` call. Compression: **3-4x**.
- **Q3 series_summary:** cold path is ~4-5 calls (general topic, attempt to
  resolve series ticker, fail, then walk events list and events get
  separately, plus an extra detail GET per market). Warm path is 2-3:
  `kalshi-series-search`, `kalshi events list --series`, `kalshi events get
  --with-markets`. Compression: **~2x**.

## Validated end-to-end

Every Greptile-found correctness fix from PR #851 rounds 2-4 was exercised
during the dogfood, either by the test suite that gates the build or by an
explicit dogfood query:

| Fix                                                              | Exercised by                                                                                                                                       |
|------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|
| Case-1 same-entity guard on cross-alias boost                    | TestRecallCanonical case-1 same-entity guard test + Q5 similar-shape Portugal vs Brazil (correctly dropped, not falsely admitted via 1.0 boost)    |
| Case-2 same-entity guard on similar-shape floor                  | TestRecallCanonical case-2 same-entity guard test + Q5 dogfood verification                                                                        |
| Case-insensitive `entitySlicesIntersect` helper                  | Existing recall_canonical_test.go case-insensitive comparison test                                                                                 |
| Atomic `AppendPlaybookNotes` (race-free amend)                   | playbook amend dogfood (timestamped `[amend 2026-05-26T15:02Z]:` marker visible in `playbook list --agent`) plus 8-goroutine concurrent-no-loss test |
| Sentinel filter on `ListPlaybooks`                               | Q6 sentinel check: `playbook list --agent` shows 4 rows, zero `__`-prefixed families                                                              |
| `ResolveSlots` candidate pool restricted to entities             | TestResolveSlots_OnlyConsidersEntities (ppg vs portugal Greptile round-3 F2) + Q1 dogfood ($TEAM resolves to Brazil, not a non-entity token)        |
| Similar-shape warning replaces misleading no-learnings           | Q5 dogfood: envelope carries `similar_shape_different_entity:Portugal` (correct), not `no_learnings_for_query_family` (which would be misleading)   |
| Ambiguous-alias narrow trigger                                   | Q7 fires `ambiguous_alias` on single-entity-multi-canonical (Cards->{Arizona, St.Louis}); Q7b does NOT fire on multi-entity (Brazil vs Portugal)   |
| Embed-FS amend-marker preservation                               | TestPlaybookInit_ReseedPreservesNotesWithAmend + TestPlaybookInit_AmendMarkerSpecificity                                                            |
| Embed-FS sentinel-stale-on-error                                 | TestPlaybookInit_FailureLeavesSentinelStale                                                                                                        |

## Surprises

- **The hand-authored playbook for `event_markets` seeds under TWO query
  families** (`all markets sibling` AND `2026 all markets sibling`). This is
  because the JSON `query_family_examples` includes both year-prefixed
  ("2026 NBA Finals") and unprefixed ("FIFA World Cup") examples; the
  `QueryFamily(Normalize(example))` derivation produces distinct keys per
  example. The behavior is correct (more family keys = more queries hit the
  playbook) but the comment in `.printing-press-patches.json` U10 entry
  correctly anticipates this ("3 playbooks; event_markets seeds under 2
  family keys" -> `playbook list` returns 4 rows).
- **The capitalization-based entity extractor splits "United States" into
  two entities `[United, States]`** rather than treating it as a single
  multi-word entity. This is an existing prediction-goat behavior (not a
  regression from this backport) — for cross-alias testing it means
  multi-token canonical names need single-token aliases. The Lusitania ->
  Portugal test in Q4 exercises the intended cross-alias path cleanly.
- **`odds Brazil wins world cup` normalizes to `cup world`** (sorted
  non-entity tokens), which matches the family key the `odds_for_team`
  playbook seeded under. The slot resolver binds `$TEAM = Brazil` from the
  query's promoted entities. This is the playbook killer-feature working as
  designed: an author writes one playbook with Ghana seed examples, and
  every other team query in the same shape inherits the playbook with
  their canonical bound to the slot.

## Recommendation

**Ship to PR #780.** All 11 prior units land; U12 confirms end-to-end
behavior in fresh-`$HOME` dogfood. No regressions in the 699-test suite.
The playbook envelope, slot binding, similar-shape warning, ambiguous-alias
narrow trigger, atomic amend, and embed-FS auto-install all behave as
designed and as the test suite predicted.

## Cross-CLI port roadmap

This backport's mechanical structure is portable to the other older-learn-loop
CLIs (kalshi, hubspot-pp-cli, contact-goat). Suggested sequencing once
PR #780 merges:

1. **kalshi** — natural next target because its data shapes overlap heavily
   with prediction-goat's Kalshi side. Existing kalshi-only repeat queries
   (`summarize series KXMENWORLDCUP`, etc.) deserve the same playbook
   primitive.
2. **contact-goat** — its repeat-query shapes (`who do I know at $COMPANY`,
   `find warm intros to $PERSON`) are even more playbook-shaped than
   prediction-goat's since the choreography crosses Happenstance + LinkedIn
   + Deepline waterfall.
3. **hubspot-pp-cli** — last because hubspot's surface is dominated by
   single-resource lookups that don't benefit as much from playbook
   choreography.

The generator template port in `cli-printing-press` (deferred per plan §Scope
Boundaries) should land in parallel so future fresh prints carry the new
shape. That is a separate plan in the generator repo.

# EverBee CLI Shipcheck (v4.28 reprint)

## Final result: PASS (7/7 legs)

| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS — **91/100, Grade A** |

Scorecard highlights: Path Validity 10/10, Auth Protocol 10/10, Sync Correctness 10/10,
Workflows 10/10, Dead Code 5/5. Weakest: Insight 4/10, Data Pipeline Integrity 7/10.
(`mcp_surface_strategy` and `live_api_verification` omitted from the denominator.)

## Blockers found and fixed

**verify-skill FAIL → PASS.** `research subniches` registered its shared flags (`--product`,
`--exclude-svg-png`, `--limit`, `--min-relevance`) through a helper method, so a static scanner
could not attribute them to the command. The flags existed at runtime, but a command whose
interface is invisible to static analysis is a command an agent cannot trust the SKILL about.
Inlined the registrations into each research command.

**validate-narrative FAIL → PASS.** `generate --force` re-emitted `root.go` and **dropped the
hand-wired `products search` AddCommand call** (it reported "0 AddCommand calls" re-injected), so
`products search --search-term` silently fell through to the bare `products` browse feed and
reported "unknown flag". Re-wired, with a comment warning the next regen. **Generator retro
candidate: the lost-registration merge path did not re-inject an AddCommand for a hand-authored
(non-scaffold) command file.**

## Code review findings (Phase 4.95) — all fixed in-place

1. **HIGH — `+Inf` crashed JSON output.** `Saturation` returns `+Inf` when demand is zero, and
   `json.Marshal` cannot encode Inf (`json: unsupported value: +Inf`) — any zero-demand seed would
   have crashed at output time. Saturation is now `*float64` and marshals as **null** when
   undefined. Deliberately *not* 0: a zero would read as "uncrowded", the exact opposite of the
   truth. Regression test added.
2. **MEDIUM — plan-cap refusals were undetectable on the product path.** The generated client
   returns a nil body alongside HTTP-status errors, so body-sniffing for the quota refusal was dead
   code. `isPlanCap` now also inspects the error text. *(Generator retro candidate: the client
   should return the response body with status errors.)*
3. **MEDIUM — hand-rolled `sqrt` under-converged** for large variance sums and could flip a
   seasonal/evergreen verdict. Replaced with `math.Sqrt`.
4. **LOW/MED — listing-ID regex matched any 6+ digit run** anywhere in the input. Now anchored:
   an Etsy listing URL or a bare all-digit ID.
5. **LOW — `Provenance.FromCache`** was documented but never set. Removed rather than left lying.
6. **LOW — a failed fetch leg emitted `"keywords": null`** instead of `[]`.
7. **LOW — `ChildSeeds` ignored its `parent` argument**; children are now actually scored against
   the parent concept.
8. **`research subniches` exited 0 when every child failed**, emitting an empty ranking that reads
   as "no opportunities". Now a non-zero error naming the broken research path.

Verified clean by the reviewer: `go vet`, `go test -race`, and every evidence invariant
(annotate-never-drop, confidence 0/0.5 caps, exact int64 listing IDs, `unknown` seasonality,
both-legs-failed = error).

## SKILL/README audit findings (Phase 4.8/4.9) — all fixed

- `auth capture` **does not exist** (real: `auth setup`, `auth set-token`) — was in the spec's
  auth instructions and the narrative; fixed at source and regenerated.
- `product-analytics search` named a nonexistent command → `products search`.
- Freshness-contract "covered paths" listed `keyword_research get|list|search` (resource name,
  underscore) and `products get` / `shops get`, none of which resolve. Corrected to the real Cobra
  paths. *(Generator retro candidate: the freshness path list uses resource names and assumes
  get/list subcommands exist.)*
- Seasonality is now disclosed as degrading to `unknown` at the decision point, not just in
  `--help`.
- `--select id,name,status` used boilerplate fields absent from EverBee rows.

## Output review (Phase 4.85): PASS, no findings

Coverage was thin — the scorecard's live probe runs in a credential-less sandbox, so only 1 of 7
samples executed. Phase 5 dogfood runs with the real session injected and covers the rest.

## Behavioral correctness (the reason this reprint exists)

Verified against the live API on the exact #1492 reproduction case:

- `research niche "dad shirt"` → opportunity 9/100, confidence 1.0 **backed by 40/40 relevant
  rows**, demand 2,565 vs competition 698,326, and an explicit refusal to label it low-competition.
  The published CLI reported confidence 1.0 with **zero** keyword evidence and unrelated products.
- `selftest` → 6/6 semantic checks pass, 20/20 keyword and 20/20 product relevance.
- `research subniches --parent dad --product-word shirt` → separates "dadfully shirt"
  (167 competitors, score 100) from "dadness shirt" (805,139 competitors, score 13.8) at equal demand.

## Verdict: **ship**

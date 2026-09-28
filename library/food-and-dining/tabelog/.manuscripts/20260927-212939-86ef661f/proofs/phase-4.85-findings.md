# Actual-output plausibility review

Result: **PASS — no findings**. Three eligible passing canonical samples were assessed under the printing-press-output-review skill's four checks.

Canonical command:

```text
<run>/toolchain/bin/cli-printing-press scorecard --dir <run>/working/tabelog-pp-cli --research-dir <run> --live-check --json
```

`<run>` is `/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f`. The ready canonical binary SHA256 supplied by its owner was `6f2964ba9184c9bfc1252681dff0d7d03d6846cfc08bb1f4985b69fbd9d26d28`.

Evidence: [phase16-livecheck.json](phase16-livecheck.json), with empty [stderr](phase16-livecheck.stderr). Scorecard exited 0; `ran_at` was `2026-09-27T16:16:33.198538Z`. It evaluated three features, all passing, and skipped two. The fixture home was seeded from genuine public Ginza bar-listing and SAMBOA detail responses, recorded in [live-fixture-seed.json](live-fixture-seed.json).

Eligible samples reviewed:

| Command | Assessment |
| --- | --- |
| `lists compare tokyo-bars --agent` | Saved identities, personal notes, listed versus review budgets, retrieval times, and listing/detail evidence are coherent. The venue `url` fields are canonical detail links; a listing `source_url` correctly identifies provenance. |
| `lists alternatives tokyo-bars --for 13005012 --match area,category --meal dinner --budget-max 5000 --agent` | Two saved alternatives match the exact verified Ginza area and `Bar` category. Their known dinner upper bounds, 4,999 and 3,999 yen, satisfy the 5,000-yen ceiling. Reasons and matched/unmatched/unevaluable counts agree; the anchor is excluded. |
| `lists audit tokyo-bars --require hours,payment,reservation,dinner_budget --max-age 24h --agent` | The detail-backed anchor's unknown listed dinner budget is distinguished from review-derived budget evidence. Listing-only candidates' hours, payment, and reservation facts are marked `detail_not_fetched`. The findings visibly distinguish those states. |

Four checks:

1. **Semantic intent:** alternatives and audit match their explicit constraints. Compare has no free-text query to assess. Public seed outputs supplied context for privacy-redacted names; the canonical passing samples remain the review evidence.
2. **Formatting:** no visible mojibake, raw HTML entities, or malformed venue links. Japanese source text is readable. `<redacted>` is privacy scrubbing, and `…[truncated]` is the scorecard's bounded capture marker, not a product-output defect.
3. **Aggregation:** not applicable; none of the eligible invocations requests a source/site/region CSV or origin fan-out. The alternatives response accounts for both evaluated saved candidates.
4. **Ordering/ranking:** these saved-list commands do not claim a quality ranking. Visible ordering is consistent with the saved-list sequence; no implausible sorting or fallback appears. Live `find` ranking is outside these sampled novel features.

Coverage limit: `lists add` and `lists refresh` were skipped by the canonical harness with `mutating example requires --allow-destructive`; they were not assessed as passing samples. The compare and audit captures are bounded to about 4 KiB. No product changes, phase receipts, or additional live probes were made for this review.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---

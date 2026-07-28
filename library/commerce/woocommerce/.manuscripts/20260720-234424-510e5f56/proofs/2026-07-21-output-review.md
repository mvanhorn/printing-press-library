# WooCommerce CLI — Agentic Output Review

## Result

`PASS — no findings`

- Reviewed all 8 passing entries from the polish live-check sample.
- Query-intent review was not applicable: none of the sampled examples accepts a free-text query.
- No raw HTML entities, mojibake, malformed URLs, or inconsistent output shapes were present.
- No sampled command requested a CSV list of sources, so there was no fan-out source-drop case to assess.
- Ranking review found no implausible ordering signal; empty local-mirror results included explicit sync or widen-window guidance.
- Catalog snapshot output clearly disclosed truncation at the configured page cap, and catalog diff clearly disclosed the lack of two comparable snapshots.

## Structured result

```yaml
---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
```

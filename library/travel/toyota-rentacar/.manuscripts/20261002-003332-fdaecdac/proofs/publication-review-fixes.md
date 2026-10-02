# Publication review fixes

Greptile’s initial review identified two valid cases. Toyota agent output now preserves JSON, provenance, checklist and uncertainty fields when quiet is requested. Class filtering now operates on the complete context-checked source offers before the requested return limit; source counts and availability are retained. Both fixes are indexed in `toyota-agent-json-and-filter-before-offer-limit.json`.

Focused regressions cover the real agent/quiet handoff and selected fields, a later class with limit one, typed absent-class errors, and limit validation before network access. The complete Go suite, build, vet, reachable vulnerability scan, skill check and publish validation passed. Full live dogfood reran successfully with 127 executed checks and 97 explicit skipped/unverified rows. An additional real C2 query with limit one and the computed agent/quiet handoff both passed; their sanitized summary is `review-fix-live-samples.json`.

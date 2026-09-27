# Review follow-up verification

Two substantive findings were reproduced and fixed: closure text must not create positive recurrence, and unknown listing city/category facts must reach bounded detail verification. Known conflicts are rejected before normalization and consume no detail budget; final filters remain strict. Search stays listing-only.

Weekday tokens distinguish the 曜日 suffix from Sunday. Monthly/nth-week rules remain unresolved rather than inventing Monday. Calendar dates/annotations are masked before recurrence parsing. Closure-only rules exclude known closed days without confirming all others; explicit daily/occurrence/positive weekly schedules can confirm activity. Bare 毎日曜/毎日曜日 means Sunday, not daily; 月曜を除く and 土日を除く are negative rules.

The JSON-key finding was a false positive: Go permits interpreted string literals as struct tags (https://go.dev/ref/spec#Struct_types). The independent JSON marshal regression passed before fixes and verifies snake_case keys without PascalCase alternatives. Tags now use conventional backticks for readability; behavior is unchanged.

Independent regressions reproduced the original failures, then passed. Final integrated verification: 11 test packages passed; six additional packages have no tests. Vet, CLI/all-package builds and pinned reachable-only vulnerability validation passed. Real-source E2E passed eight top-level tests and 16 subtests, including regions, categories, dates, city resolution, free/indoor constraints, starts/ends, wrong-year and wrong-prefecture negatives. Full tool-owned live matrix again passed 50/50 executed checks with 40 explicitly disclosed skips. No acceptance marker was hand-authored or modified.

All original proofs remain historical records. This follow-up and the refreshed phase5 marker describe the current source. Raw captures/caches/binaries remain local; host paths in these public proofs are placeholders.

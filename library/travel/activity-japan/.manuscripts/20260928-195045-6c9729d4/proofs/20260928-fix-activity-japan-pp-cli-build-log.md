Manifest transcendence rows: 5 planned, 5 built. Phase 3 will not pass until all 5 ship.

# Activity Japan Phase 11 build log

Run `20260928-195045-6c9729d4`; staged checkout `working/activity-japan-pp-cli`. All six approved source capabilities and all five novel workflow command paths resolve to real Cobra leaf help:

| Manifest row | Shipped command | Evidence |
| --- | --- | --- |
| Date-fit scan | `experience dates` | Two live Kyoto dates returned distinct option 176511 prices, 2,000/1,800 JPY, and compact session-state summaries. |
| Constraint-aware shortlist | `experience compare` | Live Kyoto/Okinawa and Osaka/Kyoto shortlists returned explicit age, party and budget verdicts; unpriced fees keep affordable-looking options unknown. |
| Price-basis report | `experience price` | Live Kyoto adult option, Osaka sushi adult/child/infant options, and Fukuoka pair option retain IDs, units, ages and selected-date prices. |
| Language-surface check | `inventory languages` | Live EN/JA plan sitemaps returned 11,331 and 17,942 IDs and separate canonical URLs; a second run used the bounded cache. |
| Handoff readiness | `experience brief` | Live Kyoto packet returned compact price/session summaries, meeting facts, unresolved fields and canonical URL. |

Also implemented `experience detail`, `experience sessions`, `experience check`, and `experience handoff`. Source interpretation lives in `internal/activityjapan/`; the hand-authored novel hook and bound source transport live in `internal/cli/activity_japan_*.go`. The five generated TODO scaffold files now delegate to the real implementations; they have no TODO paths or scaffold annotations. All novel commands declare read-only MCP hints and a live data source.

Focused tests cover Tokyo date bounds, participant-price basis, child/infant exclusion from adult subtotals, status transitions, Japanese-name partial failure, source identity/HTML failure, session ID preservation, group-stock refusal, and single-envelope `--select`. `go test ./internal/activityjapan ./internal/cli` passed. Live read-only checks passed for Kyoto chopsticks 62375, Osaka sushi 62061, Okinawa kayaking 2044, Fukuoka pair kayak 64974, and Osaka sumo 62699. An invalid plan ID failed with an explicit HTML/schema error. The stock recheck for 62375 and 62699 preserved participant quantity, session ID, source result and `reservation_confirmed:false`.

The Phase 11 dogfood gate found all 5/5 planned novel features with no missing or stub implementations. It still reports generator-level unused `maxAge`, five unused helpers, and generic sync Upsert; these do not alter the approved read-only source commands and will be assessed in shipcheck.

Deferred source access: independent destination/category search listing HTML remains WAF challenged even after a verified Firefox search page and a user-authorized temporary cookie replay. The site-scoped HAR and saved HTML were scrubbed; only non-secret request metadata and browser/source observations remain in `discovery/`. The contracted partner API requires an NDA and commercial agreement. Search is not advertised as a shipped CLI command.

Source fields left explicitly unknown when absent or ambiguous: original operator name, spoken guide languages, pickup location, mandatory fees and optional extras, tax inclusion, exact activity minutes, check-in lead time, weather dependence, and group-priced stock count meaning. Source zero party bounds are preserved as raw values and interpreted as unknown; `basic_min_passenger_count` remains raw because its semantics were inconsistent across plans.

Generated limitations: the raw endpoint mirror emits website JSON without the domain interpretation above; the generic scaffold initially contained five TODO commands and automatic documentation syncing duplicated sections. The hand-authored workflow and documentation carry the traveler-facing contract.

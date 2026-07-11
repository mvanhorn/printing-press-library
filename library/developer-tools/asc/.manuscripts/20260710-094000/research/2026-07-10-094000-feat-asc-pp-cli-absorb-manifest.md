# asc-pp-cli Absorb Manifest

Landscape: every existing ASC tool (fastlane spaceship/pilot, ittybittyapps, rorkai asccli, pofky/asc-mcp, appstoreconnect PyPI, node-app-store-connect-api) is **single-app and CI-shaped**. We match their read surface AND add a cross-app cockpit none of them has.

## Absorbed (match or beat every read feature that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List apps | spaceship / pofky `list_apps` | `asc apps list` | `--json`/`--select`/`--csv`, SQLite cache |
| 2 | Show app | kyledecot `.app(id)` | `asc app show <id\|bundleId>` | bundleId or numeric id |
| 3 | List builds + processingState | fastlane pilot | `asc builds list --app <id>` | typed state, `--limit` |
| 4 | List versions + review state | spaceship get_live_version / pofky `review_status` | `asc versions list --app <id>` | appStoreState enum surfaced |
| 5 | Show version (+ submission) | spaceship | `asc versions show <id>` | includes appStoreVersionSubmission |
| 6 | List customer reviews | pofky `list_reviews` | `asc reviews list --app <id>` | rating/territory/sort, `--json` |
| 7 | List beta groups | ittybittyapps | `asc beta groups [--app]` | agent-native output |
| 8 | List beta testers | fastlane | `asc beta testers [--group]` | agent-native output |
| 9 | Sales/downloads report | appstoreconnect PyPI | `asc sales report` | gzip-TSV decoded → rows/JSON (needs `ASC_VENDOR_NUMBER`) |
| 10 | App review details | rorkai | `asc review-details show <id>` | — |

## Transcendence (only possible with a local all-apps read model)
| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|--------------------------|
| 1 | Fleet cockpit — one-glance board across ALL apps: review state · latest build+processingState · rating · **action-needed** flag | `cockpit` | Requires fanning every app across apps+builds+versions+reviews and folding into one row; no tool renders the fleet at once |
| 2 | In-flight review board with **SLA aging** — versions stuck WAITING_FOR_REVIEW/IN_REVIEW ranked by how long | `pipeline` | Requires ordering versions across apps by `createdDate` age; no tool ages review-state across apps |
| 3 | Traction leaderboard — downloads (sales) + rating trend per app, WoW movers, rating-drop flags | `traction` | Requires joining salesReports + customerReviews per app and ranking; pofky `daily_briefing` only summarizes |
| 4 | Fleet feedback pulse — newest written reviews across all apps, one stream | `reviews recent` | Requires merging per-app review streams sorted by createdDate |
| 5 | Action-needed rollup — latest build FAILED/INVALID + pending/absent submission + METADATA_REJECTED → one fleet blocker list | `blockers` | Requires correlating build state + submission state + appStoreState across apps |

Notes / honest edges:
- **`traction` downloads half depends on `ASC_VENDOR_NUMBER`** + the salesReports gzip-TSV path (hardest hand-wired piece). Ships with rating/review-trend leaderboard solid; downloads gated on the vendor number being set (honest "set ASC_VENDOR_NUMBER" message otherwise).
- Analytics Reports API **dropped from v1** — it needs a POST to initiate a report request (a write); out of a read-only slice.
- Multi-team: v1 = one team via the `ASC_*` env trio; `--team`/profile switch is a documented fast-follow, not a stub in the surface.

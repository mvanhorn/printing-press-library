# BuildAlert Novel Features — Brainstorm + Cut

## Customer model

**Persona A: Bazil — the ZAZU operator running dual lead pipelines**

*Today (without this CLI):* Bazil already runs ZAZU — Harrow-first-class scraping plus an Idox CLI for Brent/Ealing/Croydon/Greenwich/Lambeth, all writing into `bd-mirror.sqlite` keyed on `(council_slug, reference)`. He pays BuildAlert as a hedge: BuildAlert covers 400+ councils where ZAZU only covers 6, but BuildAlert's web dashboard is a black box — he can't tell which BuildAlert leads ZAZU already has, can't pipe BuildAlert leads into his Telegram-manual-send flow, and re-keys data by hand or in copy-paste sessions. He has the BuildAlert tab open next to the ZAZU SQLite browser.

*Weekly ritual:* Every morning before standup-with-himself: pull yesterday's planning applications in ZAZU's covered councils, then open BuildAlert dashboard, manually scan for leads in non-ZAZU councils, and decide who gets a letter. Sunday: reconcile BuildAlert £2-letter spend against ZAZU Telegram sends to make sure no homeowner is being mailed twice.

*Frustration:* He cannot answer "which BuildAlert leads are NOT in `bd-mirror.sqlite` yet?" without a manual eyeball-diff. Every duplicate letter is £2 wasted AND a credibility hit with a homeowner.

**Persona B: The single-pipeline BuildAlert subscriber who wants offline triage**

*Today (without this CLI):* A loft-conversion specialist who pays for BuildAlert Premium, logs in daily, applies the same three filters (Loft_Conversion + 30 miles + £50K+), eyeballs the top 20 results, clicks into the few that look promising. They have no local database; their "history" is BuildAlert's transactions page.

*Weekly ritual:* Daily lead browse. Weekly: skim transactions tab to total up letter spend. Monthly: look at ROI tracking to see which letters got replies.

*Frustration:* They can't grep across the last 90 days' leads to find "the one in Edgware with the rooflights and dormer" they remember seeing. BuildAlert's web UI shows only the current filter window. No offline search, no SQL, no CSV export.

**Persona C: The cost-tracking small-shop owner who hates surprise spend**

*Today (without this CLI):* Runs a small extensions firm, uses BuildAlert plus an accountant. Their accountant asks every quarter "what was the £214 charge from BuildAlert in March?" — they go to the transactions tab, screenshot each row, paste into a spreadsheet. They cannot answer "what was my £/reply this quarter?" without manual joining of transactions + ROI tracking.

*Weekly ritual:* Friday afternoon spend check. Quarter end: ROI report for spouse/business-partner.

*Frustration:* BuildAlert's ROI page shows aggregate charts but not per-letter cost vs. per-letter outcome joined. "Did the £2 I spent on Reference WAL/123/4567 actually convert?" is a hand-stitch.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline verdict |
|---|------|---------|-------------|---------|--------|----------------|
| C1 | ZAZU-aware diff | `buildalert-pp-cli zazu-diff --zazu-db <path>` | List BuildAlert leads whose `internalUniqueReference` is NOT present in ZAZU's `bd-mirror.sqlite` `applications(council_slug, reference)` | A | (c) cross-entity, (e) briefing | Keep |
| C2 | ZAZU intersection report | `buildalert-pp-cli zazu-overlap --zazu-db <path>` | Show counts and sample of leads in BOTH systems by council | A | (c), (e) | Collapsed into C1 (`--mode overlap`) |
| C3 | Council coverage gap map | `buildalert-pp-cli coverage --zazu-db <path>` | Aggregate leads by council across both stores; flag asymmetric coverage | A | (c), (e) | Keep |
| C4 | Duplicate-letter guard | `buildalert-pp-cli letter-conflict --zazu-db <path>` | Cross-reference `letterBeenSent=true` against ZAZU's Telegram-send log | A, C | (c), (e) | Keep |
| C5 | Spend ledger by council/project-type | `buildalert-pp-cli analytics --type transactions --group-by council` | Aggregate £2-letter spend, grouped | C | (b), framework | Keep |
| C6 | Cost-per-reply joiner | `buildalert-pp-cli roi-per-lead` | Join `transactions` × `tracking` × `applications` for per-lead ROI | C | (a), (c) | Keep |
| C7 | Offline FTS history search | `buildalert-pp-cli search "edgware rooflights" --type leads` | Already in absorb manifest #12 | B | absorb | Cut — duplicates manifest |
| C8 | Value-band reasoning extractor | `buildalert-pp-cli leads list --select reference,estimationValueBand --json` | Flag composition, not novel | B | absorb | Cut |
| C9 | Planning-portal URL opener | `buildalert-pp-cli open <reference>` | xdg-open the planning URL | A, B | (b) | Cut — wrapper |
| C10 | Distance-radius re-filter offline | `buildalert-pp-cli nearby --postcode HA1 --radius 10` | Haversine on local lat/lng | B | (b), (c) | Keep |
| C11 | New-since-last-sync delta | `buildalert-pp-cli tail --resource leads --since 24h` | Framework `tail` | B | framework | Cut |
| C12 | Council-slug normalizer | `buildalert-pp-cli leads list --zazu-slugs` | Maps `internalUniqueReference` to ZAZU shape | A | (e), (c) | Absorbed into C1/C3/C4 |
| C13 | Subscriber-only field gate | `buildalert-pp-cli doctor --check-subscriber` | Probe subscriber-level access | A, B, C | (b) | Cut — one-time use |
| C14 | Pending-letter worklist | `buildalert-pp-cli pending-letters` | `canSendLetter=true` AND not in ZAZU sends | A | (c), (e) | Keep |
| C15 | Template inventory cross-check | `buildalert-pp-cli templates list` | Already in manifest #7 | B | absorb | Cut |
| C16 | Stale-sync staleness report | `buildalert-pp-cli stale` | `hintIfStale` already emits this | A, B, C | framework | Cut |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | ZAZU diff (BuildAlert leads missing from ZAZU) | `buildalert-pp-cli zazu-diff --zazu-db <path>` | 10/10 | hand-code | Left-anti-join local `applications` against ZAZU `bd-mirror.sqlite` on `(council_slug, reference)` derived from `internalUniqueReference`. Calls `hintIfUnsynced(cmd, db, "leads")` before reading. | Brief Top Workflow #6 explicit; user memory confirms ZAZU keys `(council_slug, reference)` matching BuildAlert's `internalUniqueReference`. |
| 2 | Pending-letter worklist | `buildalert-pp-cli pending-letters --zazu-db <path>` | 10/10 | hand-code | Local SQL: BuildAlert leads where `canSendLetter=true` AND `letterBeenSent=false` AND `(council_slug, reference)` not in ZAZU `telegram_sends`. Calls `hintIfUnsynced` for `leads`. | Brief Build Priority #4; user memory: ZAZU's Telegram-manual-send is the parallel outreach surface. |
| 3 | Duplicate-letter guard | `buildalert-pp-cli letter-conflict --zazu-db <path>` | 9/10 | hand-code | Inner-join BuildAlert `letterBeenSent=true` rows with ZAZU's send log on `(council_slug, reference)`; emit collisions. | £2/letter pricing makes duplicates costly; user memory: ZAZU also sends letters via Telegram. |
| 4 | Council coverage gap map | `buildalert-pp-cli coverage --zazu-db <path>` | 9/10 | hand-code | Group-by `council_slug` across BuildAlert local mirror and ZAZU `bd-mirror.sqlite`; produce per-council volume delta. | BuildAlert covers 400+ councils, ZAZU covers 6 (Harrow + Idox 5). |
| 5 | Spend ledger by council/project-type | `buildalert-pp-cli analytics --type transactions --group-by council` | 9/10 | spec-emits | Generator emits `analytics` over synced `transactions` grouping by the specified field. | Brief Top Workflow #5; £2/letter is the BuildAlert-defining content pattern. |
| 6 | Per-lead ROI joiner | `buildalert-pp-cli roi-per-lead --zazu-db <path>` | 8/10 | hand-code | Three-way local join `transactions` × `tracking` × `applications` keyed on lead reference; per-lead cost/reply/work-won. Optional `--zazu-db` adds ZAZU send-log column. | Brief Data Layer enumerates `letters`, `responses`, `applications` as separate entities; absorb manifest #9 surfaces aggregates only. |
| 7 | Offline radius re-filter | `buildalert-pp-cli nearby --postcode HA1 --radius 10` | 8/10 | hand-code | Haversine on local mirror's `longitude`/`latitude` against a postcode-to-lat/lng lookup; returns leads within radius without an API call. | BuildAlert's defining filter is "radius from postcode"; persona B re-filters constantly. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C2 ZAZU intersection report | Strict subset of C1 — same join, opposite predicate. Folded in as `--mode overlap` flag. | C1 zazu-diff |
| C7 Offline FTS history search | Already covered by absorb manifest row #12 (`search "loft" --type leads`). | framework `search` |
| C8 Value-band reasoning extractor | `--select` and `--json` already expose this; not a new command. | absorb manifest leads list |
| C9 Planning-portal URL opener | Thin wrapper over one JSON field; no leverage. | absorb manifest leads list with `--select` |
| C11 New-since-last-sync delta | Framework `tail --resource leads --since 24h` already does it. | framework `tail` |
| C12 Council-slug normalizer | Not a standalone user command; absorbed as internal helper inside C1/C3/C4. | C1 zazu-diff |
| C13 Subscriber-only doctor check | One-time per session, not weekly; better as a `--check-subscriber` flag on `doctor`. | framework `doctor` |
| C15 Template inventory cross-check | Duplicates absorb manifest row #7. | absorb manifest letter-templates list |
| C16 Stale-sync staleness report | `hintIfStale`/`hintIfUnsynced` helpers already emit this. | framework hint helpers |

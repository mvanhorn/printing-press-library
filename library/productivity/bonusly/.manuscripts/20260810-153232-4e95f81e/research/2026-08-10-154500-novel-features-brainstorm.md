# Bonusly Novel Features Brainstorm (audit trail)

Subagent run 2 of 2. Run 1 is superseded — it re-proposed an already-absorbed feature (`give`) as a transcendence survivor, dropped 2 of 4 seeded personas without explanation, omitted the Evidence column and 4-dimension score breakdown, and never showed Pass 3's required written reasoning. This is the corrected, accepted output.

## Customer model

**Persona 1: The Team Lead (Alex)**
- **Today (without this CLI):** Alex spends every Friday morning in the Bonusly web dashboard, manually scrolling through the recognition feed. They have open tabs for their team's directory and a spreadsheet tracking monthly "bonus pool" utilization vs. remaining balance. They struggle to correlate recognition with specific project milestones.
- **Weekly ritual:** Audit recognition patterns to identify quiet contributors, ensure the team's monthly budget is fully distributed to reward high performance, and prepare for 1:1 meetings.
- **Frustration:** The web interface doesn't allow filtering by custom date ranges across the whole team; Alex spends excessive time manually calculating team spend goals.

**Persona 2: The High-Performing IC (Jordan)**
- **Today (without this CLI):** Jordan is highly active, regularly giving points to peers to encourage specific behaviors. They keep a sticky note of who they have already recognized this month to avoid "over-recognizing" one person.
- **Weekly ritual:** During their Friday wrap-up, Jordan gives out their remaining monthly points balance to peers who helped them achieve project goals.
- **Frustration:** The `+N @mention #hashtag` mini-DSL is brittle. If they mistype the mention or hashtag, points miscategorize, and Jordan can't easily audit what tags they used last month.

**Persona 3: The Reflective IC (Sam)**
- **Today (without this CLI):** Sam is often recognized for deep-work migration tasks but struggles to recall exactly *why* they were recognized months later when writing self-reviews. They scroll back through the public feed, but Bonusly's search is company-wide context, not "recognition I personally gave or received."
- **Weekly ritual:** Searching for specific project-based recognitions to archive for annual review prep.
- **Frustration:** No personal archive or private search; everything is buried in the public firehose.

**Persona 4: The Culture Analyst (Taylor)**
- **Today (without this CLI):** Taylor is invested in company culture and curious which values are actually being practiced — is engineering rewarding #collaboration or #speed more?
- **Weekly ritual:** Monitoring the "pulse" of company values by manually tallying hashtags in the main feed.
- **Frustration:** No non-admin reporting. Cannot see hashtag/department trends without exhaustive manual tallying from the web feed.

## Candidates (pre-cut)

1. **Recognition Budget Tracker** (Persona 1 | cross-entity local query) — `recognition audit` — aggregates team recognition totals against monthly budget. Keep.
2. **Anniversary/Milestone Monitor** (Persona 2 | service-specific content pattern) — `feed --type celebration` — filters synced feed for celebration-type entries. Keep.
3. **Historical Burn Rate** (Persona 1 | cross-entity local query) — `balance --history` — graphs points burn rate from local balance snapshots over time. Keep.
4. **Personal History Search** (Persona 3 | persona-driven) — `recognition search-mine` — FTS5 over a local sync of the caller's own given+received recognition, distinct from the company-wide feed search. Keep.
5. **Department Value Tag Audit** (Persona 4 | service-specific content pattern) — `analytics values --dept <name>` — aggregates hashtag frequency per department locally. Keep.
6. **Redemption Forecast** (Persona 2 | cross-entity local query) — `redemptions forecast` — simple linear projection from local redemption history. Keep.
7. **Direct Report Recognition Gap** (Persona 1 | cross-entity local query) — `recognition gap --manager <id>` — joins org-tree direct reports against recognition-received history to flag neglected teammates. Keep.
8. **Full Recognition Feed Sync** (general | service pattern, foundational) — `sync --resources recognition` — Keep as foundation, not as a differentiator (see notes below).
9. **Fuzzy User Search** (general | identity) — `users search` — Killed: this is the generic framework `search --type users` capability with no local differentiation; re-implementing it as a "novel" command would be a wrapper, not transcendence.
10. **Admin Participation Report** — killed, requires `recognition:administer`/`reports:administer` (not available to this user tier).
11. **User Mass-Delete** — killed, requires `user:administer`.
12. **Quick Balance Check** — killed, thin 1:1 wrapper around `getPointsBalance` with no added value (already absorbed as row #21).

## Pass 3 reasoning

- **Recognition Audit** — Weekly use: high (Alex's core Friday ritual). Wrapper-vs-leverage: no, this is a local join of synced feed + department headcount, not a single API call. Transcendence proof: local SQLite join. Sibling killed: none directly, closest is Burn Rate (kept, different dimension — spend-by-team vs balance-over-time). Buildability: hand-code. Long-description: none needed (no overlapping sibling).
- **Personal History Search** — Weekly use: moderate (used when writing self-reviews/prepping for 1:1s, not strictly weekly, but recurring). Wrapper-vs-leverage: no — distinct from the company-wide feed search because it's scoped to the caller's own given+received history and stays useful with zero network calls once synced. Transcendence proof: local FTS5 over a filtered subset no live endpoint returns pre-filtered. Sibling: Value Tag Audit (kept, different persona/dimension). Buildability: hand-code. Long-description: none — command name (`search-mine`) is distinct enough from company feed search to not need a redirect.
- **Burn Rate** — Weekly use: high. Wrapper-vs-leverage: no, requires diffing successive local snapshots of the balance endpoint, which the API itself does not expose as a time series. Transcendence proof: local snapshot history. Sibling: Recognition Audit (kept). Buildability: hand-code. Long-description: none.
- **Recognition Gap** — Weekly use: high for a team lead. Wrapper-vs-leverage: no, joins the org-tree endpoint against recognition-received history — neither endpoint alone answers "who on my team have I neglected." Transcendence proof: cross-entity join. Sibling: Recognition Audit (kept, different question — spend tracking vs. neglect detection). Buildability: hand-code. Long-description: none.
- **Value Tag Audit** — Weekly use: moderate (Taylor's ritual is more monthly/curiosity-driven than strictly weekly — noted as a softer case). Wrapper-vs-leverage: no, aggregates hashtag frequency across synced feed rows, which is not a single endpoint. Transcendence proof: local aggregation. Sibling: Personal History Search (kept, different scope — company hashtag trends vs. personal message archive). Buildability: hand-code. Long-description: none.
- **Redemption Forecast** — Weekly use: moderate. Wrapper-vs-leverage: no, projects from local redemption history depth, which grows more accurate the longer `sync` has run — must be described honestly as a simple linear projection, not a sophisticated forecast, to avoid overselling. Transcendence proof: local history regression. Sibling: Burn Rate (kept, points-giving vs. points-spending — different data). Buildability: hand-code. Long-description: none.
- **Full Feed Sync** — Weekly use: high, but as infrastructure invoked implicitly by every other feature, not as a standalone differentiator a user reaches for by name. Wrapper-vs-leverage: this IS the generated `sync` command — every printed CLI has one. Transcendence proof: weak on its own (score reflects this — see below). Sibling: none. Buildability: spec-emits.

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Recognition Audit | `recognition audit` | 9/10 (Domain 3 / Pain 3 / Feasibility 2 / Research 1) | hand-code | Joins synced recognition feed against department headcounts (absorbed row #6) to compute spend-vs-budget per team, entirely offline | Absorbed manifest row #6 (`listDepartments` headcount denominator); brief's Persona 1 (team lead, no admin Participation Report access) | none |
| 2 | Personal History Search | `recognition search-mine` | 8/10 (Domain 3 / Pain 2 / Feasibility 1 / Research 2) | hand-code | FTS5 index over a local sync scoped to the caller's own given+received recognition rows, searchable with zero network calls | Brief Persona 3 (Sam, "no personal archive or private search"); Product Thesis section (no non-admin export path today) | none |
| 3 | Burn Rate | `balance --history` | 8/10 (Domain 2 / Pain 3 / Feasibility 1 / Research 2) | hand-code | Diffs successive local snapshots of the points-balance endpoint (absorbed row #21), which the live API only ever returns as a single point-in-time value | Absorbed manifest row #21 (`getPointsBalance`); brief's Top Workflow #2 (month-end forfeiture planning) | none |
| 4 | Recognition Gap | `recognition gap --manager <id>` | 8/10 (Domain 2 / Pain 3 / Feasibility 1 / Research 2) | hand-code | Joins the org-tree direct-reports endpoint (absorbed row #11) against recognition-received history to flag teammates not recognized within N days | Brief Persona 1 frustration; Product Thesis section (admin `adminUsersLastRecognized` does this exact thing but is `reports:administer`-gated — this is the honest non-admin approximation) | none |
| 5 | Department Value Tag Audit | `analytics values --dept <name>` | 7/10 (Domain 2 / Pain 2 / Feasibility 1 / Research 2) | hand-code | Aggregates hashtag frequency across synced feed rows, scoped by department headcount (absorbed row #6) | Brief Persona 4 (Taylor, hashtag/values-trend curiosity); absorbed manifest row #6 | none |
| 6 | Redemption Forecast | `redemptions forecast` | 6/10 (Domain 2 / Pain 2 / Feasibility 1 / Research 1) | hand-code | Simple linear projection over the caller's own local redemption history (absorbed row #38) | Absorbed manifest row #38 (`getMyRedemptions`); brief Persona 2 (Jordan, spend-tracking) | none |
| 7 | Full Recognition Feed Sync | `sync --resources recognition` | 5/10 (Domain 1 / Pain 1 / Feasibility 1 / Research 2) | spec-emits | Standard generated sync engine against the feed endpoint | Foundational — enables rows 1-6 | none |

**Editorial note carried into Step 1.5e (research.json authoring):** row 7 is kept in this manifest as an honest record of what the subagent scored, but will be excluded from `research.json`'s `novel_features` array. Every generated CLI has a `sync` command; listing it under README "Unique Features" would be padding, not a differentiator. Rows 1-6 (all hand-code, all scored >=6/10) are the real transcendence set — six features, not seven.

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Give recognition ("Smart Recognition Giver") | Already absorbed as manifest row #22 (`give`) — re-proposing a table-stakes command as transcendence would double-count it | Recognition Audit |
| Fuzzy User Search | Generic framework `search --type users` capability with no local differentiation — a wrapper, not transcendence | Personal History Search |
| Admin Participation Report | Requires `recognition:administer`/`reports:administer`, not available to this user tier | Recognition Audit |
| User Mass-Delete | Requires `user:administer`, not available to this user tier | Recognition Gap |
| Quick Balance Check | Thin 1:1 wrapper around `getPointsBalance` (already absorbed as row #21), no added value | Burn Rate |

**Note on "Anniversary/Milestone Monitor":** the subagent's Pass 2 marked this candidate "Keep" but then never carried it into either the Survivors or Killed tables in its corrected response — it was dropped without disposition, a fifth inconsistency beyond the four violations already sent back for correction. Rather than a third round-trip for one non-load-bearing item, I (the orchestrating agent, not the subagent) am resolving it here directly: `getRecognitionFeed`'s `recognition_types` filter (absorbed manifest row #26, confirmed values include `celebrations`) already covers "filter the feed to celebration-type entries" — `feed --type celebration` is a flag on an already-absorbed command, not a distinct novel command. Killed, my own call, not the subagent's. Closest sibling: Full Recognition Feed Sync.

**Post-generation rename note:** "Department Value Tag Audit"'s command as proposed by the subagent (`analytics values --dept <name>`) collides with the generator's own reserved top-level `analytics` command (a generic count/group-by/summary tool over synced data). This wasn't discoverable until generation actually ran and `newNovelAnalyticsValuesCmd`'s wiring was silently skipped. Renamed to `recognition values --dept <name>` post-generation (grouping it with the sibling `recognition audit`/`recognition gap` commands, which fits its data source too — it's also computed from the synced recognition feed). Reflected in the absorb manifest and research.json; this file is left as the historical record of what was actually proposed.

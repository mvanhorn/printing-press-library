# amazon-jobs — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Priya Nadar — new-grad / early-career SDE targeting AWS.**
- Today: reloads amazon.jobs, re-runs the same SDE+city search every morning, eyeballs for new listings, hand-copies id_icims to a spreadsheet.
- Weekly ritual: scans SDE reqs in Seattle/Austin/remote, opens promising reqs to read qualifications, tries to remember which were there yesterday.
- Frustration: no "what's new since last check"; intern/university reqs mixed in with full-time.

**Marcus Bell — competitive-intelligence recruiter at a rival firm.**
- Today: manually samples amazon.jobs to gauge Amazon/AWS expansion; no counts, tallies by hand across pages.
- Weekly ritual: estimates Amazon's open-SDE footprint by city and team for TA leadership.
- Frustration: `facets[]` is empty (no server aggregation) — can't answer "which teams/cities have the most open reqs" without scraping everything.

**Ada — autonomous labor-market research agent (LLM-driven).**
- Today: hits raw search.json per question, re-derives fields, can't join across records, hits the 10000-hit cap.
- Weekly ritual: answers analyst questions ("how many reqs mention Rust", "AWS hiring in EMEA") into a downstream notebook.
- Frustration: needs deterministic --json/--select + cross-field aggregation one call can't give; broad counts distorted by the 10000 cap.

**Devon Ruiz — senior IC career-switcher.**
- Today: wants senior IC (non-manager), non-intern roles, but those filters don't exist as server params; skims titles by eye.
- Weekly ritual: scans a couple cities for is_manager=false full-time reqs, skipping intern/university.
- Frustration: intern/manager/university/schedule not server-filterable and fields frequently null → manual filtering error-prone.

## Candidates (pre-cut)

C1 sync (c) PASS · C2 new-since diff (a+c) PASS · C3 saved searches (a) PASS · C4 stats aggregation (c) PASS · C5 teams browse (b+c) FLAG(subset of C4) · C6 skills/qualification-demand scan (b+c) PASS · C7 pipeline facet filters (b) PASS · C8 multi-location "available in" (b) FLAG(thin/speculative) · C9 closed/gone detection (c) FLAG(sibling of C2) · C10 trend/churn (c) FLAG(scope creep) · C11 raw SQL passthrough (c) FLAG(verifiability) · C12 apply-URL emitter (a) FLAG(thin wrapper).

## Survivors and kills

### Survivors (transcendence)

| # | Feature | Command | Persona | Score | Buildability | Long Description |
|---|---------|---------|---------|-------|--------------|------------------|
| 1 | Local-store sync | `sync --resources jobs --db ./jobs.db` | All | 9/10 | spec-emits | none |
| 2 | New-since diff | `new <saved-search>` | Priya | 10/10 | hand-code | Use `new` for reqs unseen since your last sync of this saved search; use `search --sort recent` for newest-by-posted_date regardless of what you've seen. |
| 3 | Saved searches | `save <name>` / `searches` | Priya, Marcus | 9/10 | hand-code | Use `save`/`searches` to manage persisted named queries and diff state; use `find`/`search` for one-off queries that store nothing. |
| 4 | Facet aggregation | `stats --by city\|state\|team\|category` | Marcus, Ada | 10/10 | hand-code | Use `stats` for counts across a structured facet; use `skills` to rank reqs by demand for a qualification keyword. |
| 5 | Qualification-demand scan | `skills "<keyword>"` | Ada | 8/10 | hand-code | Use `skills` to rank teams/cities by how many reqs demand a keyword; use `find`/`search` to retrieve reqs, `stats` to count by structured facet. |
| 6 | Pipeline facet filters | `find "term" --intern --manager --university --schedule <type>` | Devon, Priya | 9/10 | hand-code | none |

Note: subagent tagged stats/skills `// pp:data-source computed`; corrected to `local` (they read the synced SQLite store). Feature 6 folded into the hand-written ergonomic `find` command.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C5 teams browse | Strict subset of `stats --by team` | C4 stats |
| C8 multi-location "available in" | Speculative; thin over absorbed location filter | absorbed location filter |
| C9 closed/gone detection | Same diff machinery as new-since, weaker demand | C2 new-since |
| C10 trend/churn | Scope creep — needs persistent multi-snapshot history | C4 stats |
| C11 raw SQL passthrough | Verifiability flag; escape hatch, not a feature | C4 stats |
| C12 apply-URL emitter | Thin wrapper; id already in `get` output | absorbed get-by-id |

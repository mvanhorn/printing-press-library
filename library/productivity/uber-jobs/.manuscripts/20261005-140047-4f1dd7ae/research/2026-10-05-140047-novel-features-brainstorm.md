# Novel-features brainstorm: uber-jobs (full subagent response, audit trail)

Subagent: Opus via the Agent tool, read-only, first print (prior research `none`), 2026-10-05.

## Customer model

Grounding: the brief's Users line ("job seekers tracking Uber openings across markets, career coaches and recruiters, and agents driving a job-tracker pipeline"), Top Workflows 1-5, and the User Vision.

**P1. The morning tracker run.** This is the owner's job-tracker script and the agent that drives it. It already calls amazon-jobs-pp-cli once per target market and writes rows into the owner's tracker.
- **Today (without this CLI):** The script has a working Amazon leg and no Uber leg. Every community client of `/api/jobs/search/` gets in only with Chrome TLS+HTTP/2 impersonation (headstart uses curl_cffi, ats-scrapers uses httpcloak). Stock curl and python-requests get a 403 `cf-mitigated: challenge`. The old `www.uber.com/api/loadSearchJobsResults` has returned 404 since about August, which broke trackers such as FAANG-2027-Internships-Tracker. So Uber rows get added by hand from a Chrome tab. The script cannot tell which Uber rows already in the tracker are still open, when a row closed, or whether an empty pull meant "no postings" or "refused".
- **Weekly ritual:** On every run, for each ISO3 market, it calls `postings --country GBR --limit 100 --offset 0 --sort recent --base-query <kw> --json --data-source live`. It parses posted_date (Month D, YYYY), merges by id, and marks rows that are gone.
- **Frustration:** Keeping existing rows honest. Confirming that 40 tracked postings still exist takes 40 lookups. A Cloudflare refusal, or a page walk that drifted (headstart measured 100- and 200-row walks that each dropped one job), looks exactly like a closed posting.

**P2. The Monday relocation check.** A job seeker targeting Uber in two or three markets, for example GBR and NLD, who needs visa sponsorship and does not speak the local language.
- **Today:** One jobs.uber.com tab per country, sorted by date. They open every posting and scan it for sponsorship, relocation and language requirements (Top Workflow 4). Job alerts are an email subscription, and the CLI keeps those out of scope. They cannot tell what is new since last Monday without remembering what they have already seen. A date sort also misses postings that show up late: search replies are edge-cached for hours (`x-vercel-cache: STALE`, `age: 15081`, about 4.2 h). On 5 Oct, 127 of 578 postings were under 7 days old, so each week brings a fresh batch.
- **Weekly ritual:** Monday morning: see what is new in their markets since last week, drop anything with a disqualifying sentence, and shortlist the rest into their tracker.
- **Frustration:** Rereading postings they have already seen, and opening each description to find the one sentence that rules it out.

**P3. The coach's weekly market read-out.** A career coach or recruiter who tells clients which Uber teams and countries are hiring.
- **Today:** They filter jobs.uber.com by country and team and count by eye. The /en/jobs/ payload names 37 countries, 18 teams and 53 subteams, but the site only shows today's list. 27.9% of postings list 2-5 locations, so hand counts per country are wrong. They cannot answer "did Delivery open more postings in MEX this month than it closed?"
- **Weekly ritual:** Once a week, they pull counts per country and per team (open, newly posted, closed) to brief clients.
- **Frustration:** There is no history. The site does not remember what closed, so comparing what opened with what closed is impossible by hand.

## Candidates (pre-cut)

Gating: the brief has `## User Vision` and `## Codebase Intelligence`, so sources (e) and (f) are active. Prior research.json is `none`, so this is a first print and (d) is skipped.

| # | Candidate | Command | One-line description | Persona | Source | Long Description | Inline rubric verdict |
|---|-----------|---------|----------------------|---------|--------|------------------|-----------------------|
| C1 | New-since per saved search | `new <name> [--country GBR --base-query strategy --team <t>]`; `new --all`; `new --list` | The first run with filters saves the search and baselines the set of ids it matches. Later runs list postings whose ids are not in the baseline, plus baseline ids that have since closed. The baseline advances only after a complete, uncapped sync. | P2, P1 | (a) P2 frustration; (e) User Vision "saved searches with a new-since command"; (f) edge cache up to ~4.2 h, so the diff uses ids rather than a date watermark | Use this command to see postings that appeared in, or closed from, a saved search since its last complete scan. Do NOT use this command for a one-off list of the most recent postings; use 'postings --sort recent' instead. | KEEP. Local SQLite only. No LLM, no external service, read-only, about 150 LoC. Verifiable by syncing two fixture corpora. |
| C2 | Saved-search management | `searches add\|list\|rm` | Create, list and remove named filter sets in the local store | P2 | (e) User Vision | none | REFRAME. Passes every rubric check, but it is plumbing you set up once. Fold creation into C1's first run and listing into `new --list`. |
| C3 | Tracker liveness check | `check <id>...`; `cut -f1 ids.txt \| check -` | For each id, reports open, closed (with closed_on, first_seen, last_seen), never_seen, or unknown. Answers come from the last complete sync, or with `--data-source live` from one fresh full-corpus pull. | P1 | (a) P1 frustration; (c) tracker ids joined to postings history; Top Workflow 5 | Use this command to check whether a list of known posting ids is still open and when each one closed. Do NOT use this command to fetch one posting's full details; use 'get' instead. | KEEP. Local history plus the absorbed full-corpus path. Read-only, no LLM. Verifiable with fixture ids. |
| C4 | Disqualifier screen with evidence | `screen --country NLD --posted-within 7d --require "sponsorship" --exclude "fluent Dutch"` | Keeps or drops postings by phrases in the HTML-stripped description, and prints the sentence each phrase matched for every posting | P2 | (a) P2 frustration; (e) User Vision "description contains and not-contains filters"; (f) 57 of 538 Descriptions are whole HTML documents | Use this command to keep or drop postings by phrases in their description and see the sentence that matched. Do NOT use this command to find postings by keyword; use 'postings --base-query' for live results or 'search "<term>" --type postings' for the local store instead. | KEEP. Mechanical phrase match, no NLP. The evidence sentence lets the person or agent judge negations such as "unable to offer sponsorship" instead of the CLI guessing. |
| C5 | Description filter flags on postings | `postings --desc-contains X --desc-not-contains Y` | The same match as C4, added as flags on the absorbed tracker command | P2 | (e) User Vision | none | CUT. A flag on an absorbed command is not a transcendence row. Without the matched sentence it cannot tell "sponsorship available" from "no sponsorship". |
| C6 | Posted-within | `postings --posted-within 7d` | Filters on a DisplayDate window | P1, P2 | (e) User Vision; table stakes (JobSpy hours_old, LinkedIn time filters) | none | CUT as a transcendence row. It is a table-stakes filter flag with no leverage. |
| C7 | Work-pattern, contract-type and remote flags | `postings --work-pattern Hybrid --contract-type <t> --remote` | Facet filters on WorkPattern, ContractType and Remote | P2 | (b) Uber's workPatterns and contractTypes facet lists | none | CUT. Table-stakes flags. Remote is false on every sampled row. |
| C8 | Hiring stats with churn | `stats --by country\|team\|subteam [--country MEX]` | For each group: open now, posted in the last 7 and 30 days (DisplayDate), and opened and closed in the last 30 days (first_seen and closed_on). A multi-location posting counts once for each country it lists. | P3 | (c) postings joined to posting_locations and sync history; (e) User Vision "sync plus stats by category" | Use this command for per-country, per-team or per-subteam hiring counts with open, newly posted and closed columns. Do NOT use this command for a count grouped on any other single field; use 'analytics --type postings --group-by <field>' instead. | KEEP. Local SQLite join, no LLM. Verifiable against fixture counts. |
| C9 | Closed-postings feed | `closed --since 30d` | Every posting that closed in a time window, with days_open | P3, P1 | (c) first_seen and closed_on | Use this command for every posting that closed in a window across the whole corpus. Do NOT use this command for the status of specific ids; use 'check' instead. | KEEP on the rubric (local data). Goes to the Pass 3 sibling test. |
| C10 | Relist detector | `relists` | New ids whose normalized title and location set exactly match a posting that closed in the last N days | P2, P3 | (b) Oracle-origin requisition numbers; (c) | none | FLAG on verifiability. A heuristic match that cannot be confirmed from the data, and it needs months of history. |
| C11 | Pay-transparency ranges | `pay --country USA --team <t>` | Parses currency ranges out of the Salary.Description pay text | P2, P3 | (b) Uber's Salary block | none | FLAG on verifiability. Salary numbers are null on every sampled row, and the format of the pay text is unverified. |
| C12 | Same role in other markets | `elsewhere <id>` | Open postings with the same normalized title or subteam in other countries | P2 | (c) | none | LLM-dependency check: "the same role" is semantic grouping. Reframed to exact normalized title plus the same subteam, which is weak. |
| C13 | Source reconciliation | `reconcile` | Compares the id sets from jobs.uber.com and Oracle, plus the edge-cache age | P1 | (f) the same requisition numbers on both surfaces; totals 578 (HTML) vs 581 (API) vs 578 (Oracle) | none | CUT. It calls Oracle when jobs.uber.com has not refused, which breaks the User Vision rule that Oracle is "used automatically only when jobs.uber.com refuses". |
| C14 | Recommendations | `recommend <id>` | Wraps POST /api/jobs/recommendations {viewJobIds, maxJobs} | P2 | (f) the site's JS | none | CUT. A thin wrapper of an endpoint that appears only in the JS and was never captured in the HAR, so its request and response shapes are unproven. |
| C15 | Weekly digest | `digest` | The deltas for every saved search in one envelope | P2, P1 | (a) | none | Sibling of C1. Goes to Pass 3. |

## Survivors and kills

### Pass 3 answers

**C1 `new`**
1. **Weekly use:** Yes. P2 runs it every Monday, and P1 runs `new --all` on every tracker run.
2. **Wrapper vs leverage:** Leverage. No endpoint returns "new since my last look". The site keeps no per-user state apart from its email alerts.
3. **Transcendence proof:** Local SQLite. It joins the saved-search baseline id sets with postings first_seen/closed_on and the sync run log's complete flag. Comparing id sets instead of dates absorbs the ~4.2 h edge cache and the 578/581 total wobble. A posting that shows up late with an older DisplayDate still counts as new.
4. **Sibling kill:** C15 `digest` died as a duplicate of `new --all`. C2 `searches` was folded into `new`'s first run.
5. **Buildability:** `hand-code`.
   - It needs new `saved_searches` and `saved_search_ids` tables and `// pp:data-source local`. `--data-source live` is rejected with "no live equivalent; run sync first".
   - It calls `hintIfUnsynced(cmd, db, "postings")` and then `hintIfStale`.
   - Drain-first: scan the baseline ids into a map and close `rows` before querying postings.
   - The advanced baseline is written in its own transaction with no `store.Upsert` inside it, and a failed write fails the command.
   - If the last sync was incomplete, `new` still lists additions but sets `meta.baseline_advanced=false` and computes no closures.
6. **Long-description validity:** It names `postings --sort recent`, an absorbed command (manifest rows 4-5). Valid.

**C3 `check`**
1. **Weekly use:** Yes. P1 runs it on every tracker run.
2. **Wrapper vs leverage:** Leverage. The raw `jobs recently-viewed` mirror (manifest row 11) returns rows only for ids it finds, and how it handles closed ids is unverified. `check` answers from membership in a complete corpus scan and dates each closure.
3. **Transcendence proof:** Local SQLite history plus a per-id status shaped for agents. `unknown` is returned when the last scan was incomplete or refused, so a refusal never reads as `closed`.
4. **Sibling kill:** C9 `closed` is a monthly corpus-wide view that never sees the tracker's ids.
5. **Buildability:** `hand-code`.
   - It uses `// pp:data-source auto`. Live mode is the absorbed size probe plus one page sized to totalJobs. On refusal, the Oracle fallback needs only findReqs search rows, because ids are enough and no detail calls are needed. Local mode uses the last complete sync.
   - It calls the hint helpers.
   - Drain-first: read the ids into a slice, run one `IN (...)` query, drain and close it, then build the output.
6. **Long-description validity:** It names `get` (manifest row 7). Valid.

**C4 `screen`**
1. **Weekly use:** Yes. P2 runs it every Monday after `new`.
2. **Wrapper vs leverage:** Leverage. No endpoint filters on description text. The site's only text key, `search`, is a positive keyword match with no exclude.
3. **Transcendence proof:** FTS5 over HTML-stripped descriptions in local SQLite, plus an evidence column with the sentence each phrase matched. That column lets a person or agent check negations. The stripper must handle the 57 of 538 descriptions that are whole HTML documents.
4. **Sibling kill:** C5's description flags on `postings` do the same match with no evidence sentence, as a flag on an absorbed command.
5. **Buildability:** `hand-code`.
   - It uses `// pp:data-source local` and the hint helpers.
   - Drain-first: collect the matching ids from the FTS query into a slice and close `rows` before extracting sentence evidence.
   - Postings with no description, such as rows from an Oracle-fallback sync, come back as `unscreened`, never as kept.
6. **Long-description validity:** It names `postings --base-query` (manifest row 2) and the framework `search "<term>" --type postings`. Valid.

**C8 `stats`**
1. **Weekly use:** Yes. It is P3's weekly read-out. The opened_30d and closed_30d columns are null, not zero, until sync history covers the window, and meta says so.
2. **Wrapper vs leverage:** Leverage. No endpoint returns counts. The framework `analytics --group-by` groups one column of one table, so it cannot unnest posting_locations or compute churn.
3. **Transcendence proof:** A local SQLite join of postings, posting_locations, the facets ISO3 map, and first_seen/closed_on.
4. **Sibling kill:** C10 `relists` was a hiring-signal heuristic that cannot be verified and would be used monthly at best.
5. **Buildability:** `hand-code`.
   - It uses `// pp:data-source local` and the hint helpers.
   - Drain-first: drain one GROUP BY query into structs before resolving labels and ISO3 codes from the facets table.
   - `meta.distinct_postings` explains why column sums can exceed hits when postings list several countries.
   - Rows with no team are grouped under `null`, because Oracle-sourced rows carry no category.
6. **Long-description validity:** It names the framework `analytics --type postings --group-by <field>`. Valid.

### Survivors
| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------------|--------------|----------|------------------|
| 13 | New-since per saved search | `new uk-strategy --country GBR --base-query strategy` (first run saves the search and sets its baseline); `new uk-strategy --agent`; `new --all --json`; `new --list` | 9/10 (fit 3, pain 3, build 1, research 2) | P2 (also P1) | hand-code | Uses the local postings table (id, first_seen, closed_on), a saved_searches table holding each search's filters and baseline id set, and the sync run log's complete flag (unique == totalJobs and scan_cap_hit false). From these it computes the postings added to and closed from a saved search since its baseline, with no external dependencies. | Brief Table Stakes: "Not table stakes anywhere: saved searches, new-since/alerts and offline stats. Only app-level repos have them." Top Workflow 2: the baseline advances only after a complete, uncapped scan. User Vision lists it as a candidate. Reachability: edge-cached `age: 15081`, "New-since logic must tolerate hours of lag". Oracle postingDatesFacet: 127 postings under 7 days old. | Use this command to see postings that appeared in, or closed from, a saved search since its last complete scan. Do NOT use this command for a one-off list of the most recent postings; use 'postings --sort recent' instead. |
| 14 | Tracker liveness check | `check 123456 234567 --json`; `cut -f1 ids.txt \| check - --agent` | 8/10 (fit 3, pain 2, build 2, research 1) | P1 | hand-code | Uses the local postings table's first_seen, last_seen and closed_on from the last complete sync. With `--data-source live` it instead makes one size probe plus one page of GET /api/jobs/search/ sized to totalJobs, falling back to Oracle findReqs search rows, which carry ids. From either it computes an open / closed / never_seen / unknown status per id, with no external dependencies. | Top Workflow 5: "Look up one posting by id and confirm it still exists." Data Layer: "Postings not seen in a complete sync are marked closed, not deleted." Surfaces disagree within one session (578 vs 581), so a posting is closed only after a complete scan. The owner rule never retries a 403 or challenge, so a refusal has to surface as `unknown`. | Use this command to check whether a list of known posting ids is still open and when each one closed. Do NOT use this command to fetch one posting's full details; use 'get' instead. |
| 15 | Disqualifier screen with evidence | `screen --country NLD --posted-within 7d --require "sponsorship" --exclude "fluent Dutch" --json` | 8/10 (fit 3, pain 2, build 2, research 1) | P2 | hand-code | Uses the FTS5 index over HTML-stripped descriptions in the local postings table to compute kept, dropped and unscreened postings, each with the sentence every `--require` / `--exclude` phrase matched, with no external dependencies. No detail calls are needed because the listing Description is byte-identical to the JSON-LD one. | Top Workflow 4: description contains / not-contains for "sponsorship, relocation, language", plus posted-within, "with no HTML noise". User Vision lists it as a candidate. Codebase Intelligence: 57 of 538 Descriptions are whole HTML documents titled "<p> Cleaned Document </p>", and the listing Description is byte-identical to JSON-LD (headstart 3/3). | Use this command to keep or drop postings by phrases in their description and see the sentence that matched. Do NOT use this command to find postings by keyword; use 'postings --base-query' for live results or 'search "<term>" --type postings' for the local store instead. |
| 16 | Hiring stats with churn | `stats --by country`; `stats --by team --country MEX --json` | 8/10 (fit 2, pain 2, build 2, research 2) | P3 | hand-code | Uses local postings joined to posting_locations, with country names mapped to ISO3 through the facets snapshot, plus DisplayDate, first_seen and closed_on. From these it computes open, posted_7d, posted_30d, opened_30d and closed_30d per country, team or subteam, with no external dependencies. | Top Workflow 3: "answer stats by category, team or country offline". Build Priority 5. User Vision: "sync plus stats by category". 27.9% of postings list 2-5 locations. Oracle postingDatesFacet: 127 under 7 days, 314 under 30 days, 264 over 30 days. Table Stakes: offline stats exist only in app-level repos. | Use this command for per-country, per-team or per-subteam hiring counts with open, newly posted and closed columns. Do NOT use this command for a count grouped on any other single field; use 'analytics --type postings --group-by <field>' instead. |

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C2 Saved-search management (`searches add\|list\|rm`) | You set it up once and never run it weekly. `new` creates a saved search on its first run with filters and lists searches with `new --list`. | `new` |
| C5 Description filter flags on `postings` | It is a flag on an absorbed command, not a transcendence row. A bare match cannot tell "sponsorship available" from "unable to offer sponsorship" without the evidence sentence. | `screen` |
| C6 `postings --posted-within` | Table stakes (JobSpy hours_old, LinkedIn time filters) with no leverage as its own row. `screen` applies the same window locally. | `screen` |
| C7 Work-pattern, contract-type and remote flags | Table-stakes filter flags. Remote is false on every sampled row, so a `--remote` built on that field would silently return nothing. | `screen` |
| C9 `closed --since 30d` | Fails the weekly-use test: P3 would look at what closed monthly, and its counts already appear in `stats` closed_30d. P1 needs closure per tracked id, which `check` gives. | `check` |
| C10 `relists` | Exact title-plus-location matching is a heuristic that dogfood cannot verify. It needs months of history before it says anything, and P3 would use it monthly at best. | `new` |
| C11 `pay` | Salary numbers are null on every sampled row and the pay-text format is unverified, so parsed ranges cannot be checked in dogfood. No persona runs a pay survey weekly. | `screen` |
| C12 `elsewhere <id>` | "The same role in another market" is semantic grouping. The exact-title reframe catches little, and no research asks for it. | `stats` |
| C13 `reconcile` | It calls Oracle when jobs.uber.com has not refused, against the User Vision rule that the fallback runs only on refusal. It is also a diagnostic, not a weekly ritual. | `check` |
| C14 `recommend <id>` | A thin wrapper of an endpoint that appears only in the JS and was never captured in the HAR, so its request and response shapes are unproven. | `new` |
| C15 `digest` | It duplicates `new --all`, which runs every saved search into one envelope. | `new` |

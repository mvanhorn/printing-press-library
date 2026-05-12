# pubsec-tech Novel Features Brainstorm (Phase 1.5c.5 audit trail)

## Customer model

**Persona 1 — Priya, the BD analyst at a mid-size gov-tech vendor**

*Today (without this CLI):* Priya has six tabs open every morning: SAM.gov advanced search (filtered to her four IT NAICS codes), USAspending advanced search (filtered to DOD + civilian IT agencies), her CRM, her email digest from a $40K Govini seat that her boss has been threatening to cancel, the Nextgov homepage, and a Google Sheet where she pastes notice IDs by hand. She re-runs the same `requests`-based Python script every Monday morning to pull SAM postings since Friday and dedupe against last week's CSV. She can't answer "for the contracts I'm tracking, which ones expire in the next 18 months and who's the incumbent?" without exporting two CSVs and pivoting in Excel.

*Weekly ritual:* Monday morning — pull new IT opportunities posted over the weekend, cross-check against existing tracked contracts, flag any with set-asides her firm qualifies for, scan the news for mentions of competitors winning relevant work, and email a "BD digest" to her sales lead by 10am.

*Frustration:* The Monday digest takes 2.5 hours and the news cross-check is purely manual — she reads Nextgov + FedScoop headlines and tries to remember which vendors she's tracking. She frequently misses competitor wins that surface in news before they hit USAspending.

**Persona 2 — Marcus, the federal-IT trade-press reporter**

*Today (without this CLI):* Marcus covers federal IT for an industry publication. He gets pitched 30 vendor stories a week and needs to validate each: "Is this vendor actually winning work? With which agencies? How big? Is there a recent recompete?" His current flow: SAM.gov entity search → manual cross-ref to USAspending recipient profile → eyeball the FY totals. He maintains a private list of 80 vendors he watches; checking each takes ~5 minutes, so he only spot-checks before publication.

*Weekly ritual:* Wednesday afternoon — rip through his vendor watchlist looking for "what changed this week" (new awards, new opportunities they're chasing, exclusions filings, news mentions) to seed his Friday newsletter.

*Frustration:* No single command says "for vendor X, what's new across awards + opportunities + news in the last 7 days." He cobbles it together vendor by vendor.

**Persona 3 — Dana, the agentic-research user driving Claude/MCP**

*Today (without this CLI):* Dana asks her agent questions like "what did GSA spend on cloud modernization in FY24 and what's coming up?" The agent stitches together half-broken USAspending and SAM MCP tools, frequently invents NAICS codes (`541512` vs `541511` vs `518210` — agents hallucinate badly here), and can't correlate the dollar trail with the news narrative because the news is in a different MCP server entirely.

*Weekly ritual:* Whenever a question comes up — could be daily, could be ad hoc. Asks her agent to compile a "what's the federal IT picture on topic X" answer, expecting joins across spend + opportunities + news.

*Frustration:* Hallucinated NAICS/PSC codes silently produce empty result sets. The agent confidently returns "no contracts found" when the real answer was "you asked with the wrong code." She'd happily trade some breadth for guardrails that refuse to query with invented codes.

## Candidates (pre-cut)

[16 candidates generated; full table in subagent return — see Survivors and kills below for the post-cut state.]

## Survivors and kills

### Survivors (9 features, all score >=6/10)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Vendor rollup | `vendor "Leidos"` | 9/10 | Joins local `entities` (SAM) + `exclusions` + `recipients` (USAspending) + open `opportunities` + recent `articles` tagged with vendor name into one structured response | Brief Top Workflow #3 explicitly calls this out; capture-mcp's `get_entity_and_awards` is described as "shallow join" in Codebase Intelligence — we go deeper |
| 2 | Recompete radar | `recompete --naics 541512 --within 18m` | 8/10 | Local SQL on `awards.period_of_performance_current_end_date BETWEEN now() AND now()+18m`, joined to `recipients` for incumbent profile and to `opportunities` for any follow-on RFP already posted | Brief Top Workflow #6, User Vision implication list calls out "recompete radar" by name; no existing MCP tool offers this |
| 3 | News-to-contract correlation | `news --since 7d --link-contracts` | 9/10 | Deterministic name-match of `recipients.recipient_name` and `agencies.toptier_name` against `articles.title + content`, persisted in `tags` table, returned as article→{award_ids[], opp_ids[]} pairs | Brief Top Workflow #5; user vision states news↔contract correlation is "the reason this CLI exists"; no existing tool combines all three surfaces |
| 4 | Agency modernization view | `agency DOD --modernization` | 7/10 | Composes four local queries scoped to a curated IT-NAICS set: open opps for agency, IT-NAICS awards last N quarters, spend trend, news mentions | Brief Top Workflow #4; Priya persona ritual covers per-agency views |
| 5 | Weekly BD digest | `digest --since 7d --naics-profile mine --json` | 8/10 | Composes recompete + deadlines + news-correlation outputs, scoped to a user-saved NAICS profile stored in local config | Brief Top Workflow #7; MindPetal/sam-search is the prior art on digests; Priya persona's Monday ritual maps exactly |
| 6 | Explain this headline | `explain "url or headline"` | 7/10 | Looks up article by URL or fuzzy title in local `articles`, reads its `tags` rows, returns linked `awards` and `opportunities` with the matched mention spans | Brief User Vision implications list calls out "explain this headline" explicitly; no existing tool does this |
| 7 | Anti-hallucination code guard | `code resolve "cloud" --kind naics` | 7/10 | Looks up term against local `naics`/`psc` tables; on miss, returns top-K nearest by `LIKE` and trigram score and exits non-zero rather than guess | Brief Table Stakes calls out "NAICS autocomplete (anti-hallucination — agents make up codes)"; Dana persona frustration is documented; differentiator vs cliwant's permissive autocomplete |
| 8 | Set-aside eligibility filter | `opps search --set-aside-eligible-as <UEI>` | 7/10 | Reads SAM entity socioeconomic indicators for the UEI, then filters `opportunities.typeOfSetAsideDescription` to the entity's qualifying set-aside categories | Brief Top Workflow #2 lists set-aside filtering; Priya persona ritual triages by set-aside; no existing MCP cross-references entity eligibility with opportunity set-aside |
| 9 | Vendor watchlist diff | `watch vendor "Leidos" --since-last-sync` | 6/10 | Persistent watchlist in local store; on invocation, returns awards/opps/articles touching the vendor since the last recorded watch-tick, advances the tick | MindPetal/govbizops prior art on multi-NAICS polling; Marcus persona's Wednesday vendor-watch ritual maps directly |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| C9 CR window flag | Requires maintained federal CR calendar — stale-data risk; reframe as `--fy-quarter` filter on existing `awards search` | C4 agency modernization view |
| C11 Subaward food chain | USAspending exposes one level of subawards, not recursion; reframe as single-level `--rollup-by-recipient` flag on existing `awards subs` | C1 vendor rollup |
| C13 Cross-source FTS `search` | Generic three-table FTS5 is a data-layer table-stake, not transcendence; the join-with-semantics version is C3 | C3 news-to-contract correlation |
| C14 Top vendors by IT NAICS | Already covered by absorb manifest #12 (`analyze_competition`) | C1 vendor rollup |
| C15 Geographic spend by congressional district | Weak persona demand; state-level (#23) is enough | C4 agency modernization view |
| C16 FedRAMP tagging | Mechanism collapses into C3; standalone command not warranted | C3 news-to-contract correlation |

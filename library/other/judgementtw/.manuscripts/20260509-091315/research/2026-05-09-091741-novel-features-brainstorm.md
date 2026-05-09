# Novel Features Brainstorm — judgementTW

(Subagent: novel-features-subagent, three-pass output preserved verbatim for audit/retro.)

## Customer model

**Persona 1: Mei-Ling Chen, criminal defense paralegal at a Taipei law firm**

*Today (without this CLI):* Mei-Ling has six tabs open every morning: `judgment.judicial.gov.tw`'s `Default_AD.aspx` form, two FJUDKM commentary pages, a Word doc of citations, a PDF reader, and Lawsnote (which the firm now uses cautiously since the 2024 verdict). She types each case number into the WebForms search by hand, copy-pastes the 主文 and 理由 into a research memo, and re-types the JID into the URL bar to fetch the PDF. When her supervising attorney asks "has TPHM ever applied 毒品危害防制條例 §17(2) to a defendant under 20?" she cannot answer without an hour of manual filtering. She cannot download PDFs in batch — the website serves them one at a time behind a session cookie.

*Weekly ritual:* Pull every new High Court drug case from the past week, extract the sentencing range, find the cited statutes and prior cases, and assemble a precedent packet for next week's hearings.

*Frustration:* The 0–6 AM service window on the official open data API is incompatible with her workday, and the FJUD search UI returns one HTML page at a time with no JSON, no field projection, and no way to ask "which of these 47 cases cite §17(2)?"

**Persona 2: Professor Wu, empirical legal scholar at a national university**

*Today:* Wu maintains a Python+Selenium scraper (cloned from `samttoo22-MewCat/judgement-scrawler`) that she babysits weekly. It breaks every few months when ASP.NET ViewState changes; her grad student spends two days fixing it. Her corpus is a folder of 80,000 `.txt` files named by JID, with a `pandas` notebook that grep-searches them. She publishes papers on sentencing disparity across districts but cannot answer reviewer questions like "stratify by 字別" without re-running the notebook for 40 minutes.

*Weekly ritual:* Sync the past week's judgments for a study cohort (e.g., all 行政 cases involving 環境影響評估), tag them by court and year, and update her sentencing-pattern dataset.

*Frustration:* Every empirical question requires custom scripting because the data lives in 80,000 unindexed text files. She cannot easily ask "which courts cite this 大法官解釋 most often?" without writing new code.

**Persona 3: Yi-Chen Lin, investigative journalist at a digital newsroom**

*Today:* Yi-Chen tracks corruption convictions and writes a weekly column. She manually checks four court websites every Monday, searches by judge name (when she can guess at the 字別), and screenshots judgment headers for source material. She has no way to be alerted when a new ruling drops in an ongoing case she is following. She often finds out about important rulings days late from competitor outlets.

*Weekly ritual:* Check 5–10 ongoing cases for new rulings, scan the previous week's anti-corruption (貪污治罪條例) judgments, and pull commentary from FJUDKM for context.

*Frustration:* She has no watch/alert mechanism. The website has no RSS, no email digest, and the open data API is asleep when she's writing.

**Persona 4: Agent operator, Claude/MCP user doing legal-research tasks**

*Today:* The agent uses raw `WebFetch` against `Default_AD.aspx`, gets back HTML it then has to parse with regex, gives up halfway because of ViewState/iframe games, and returns "I couldn't retrieve the case." It cannot maintain a corpus across turns and cannot answer follow-ups like "of those 12 cases you found, which were appealed?"

*Weekly ritual:* On-demand: a user asks "find me Taiwan cases on X" or "summarize the latest constitutional court ruling on Y."

*Frustration:* No agent-shaped JSON surface, no field selection, no joinable corpus — every query starts from zero.

## Candidates (pre-cut)

(Full table from the subagent — 16 candidates labeled C1–C16, with persona served and rubric kill/keep notes. Killed candidates are included for audit. See "Survivors and kills" below for the cut.)

## Survivors and kills

### Force-answers applied (Pass 3)

| # | Weekly use? | Wrapper or leverage? | Transcendence proof | Sibling kill |
|---|-------------|----------------------|---------------------|--------------|
| C1 | Yi-Chen daily, Mei-Ling weekly | Leverage: diff + persistence requires store | Local store + diff against last-seen JID set | Beats C16 |
| C2 | Yes weekly for all three humans | Leverage: stored query + sync cursor | Local query + cursor table | Sibling of C1 but distinct |
| C3 | Mei-Ling weekly, Wu often | Leverage: extracted citation table + cross-judgment scan | 引用法條 pattern + local SQLite | Beats C12 |
| C4 | Mei-Ling weekly | Leverage: reverse index of citation graph | Cross-entity local join | Distinct from C3 |
| C5 | Wu weekly | Leverage: regex 主文 sentence patterns + aggregation | 主文 section parsing + local agg | Sibling kill: C14 |
| C6 | Wu monthly — soft | Wrapper-ish: same data is one sql away | Local agg | Subsumed under C5 + sql |
| C7 | Mei-Ling weekly, agent often | Leverage: per-court 字別 catalog | Aggregation over corpus | Distinct |
| C8 | Mei-Ling weekly | Leverage: chain via 案號 root + court hierarchy | Court hierarchy + local join | Distinct from C4 |
| C9 | Every command (民國 dates pervasive) | Foundational — promote to shared flag | Service-specific calendar | Demoted |
| C10 | Wu/Mei-Ling weekly | Leverage: required by ToS, by brief | Lifecycle pattern | Distinct |
| C11 | Agent every sync | Tiny but used constantly | Service-specific schedule | Standalone |
| C12 | Mei-Ling weekly | Wrapper of absorbed #8 | None | KILLED |
| C13 | Wu weekly | Leverage: only join across two sites | Cross-source join | Distinct |
| C14 | Wu monthly | Verifiability fails on judge name extraction | Extracted entity | KILLED |
| C15 | Mei-Ling weekly | Leverage: similarity by citation overlap | Local similarity | Distinct |
| C16 | Yi-Chen monthly | Below weekly bar; subsumed by C2 | None | KILLED |

### Survivors

| # | Feature | Command | Score | How It Works | Evidence | Persona-served |
|---|---------|---------|-------|--------------|----------|----------------|
| 1 | Watch a specific case for new rulings | `watch case <jid-pattern>` | 8/10 | Polls FJUD search by court+案號 root; diffs returned JIDs against local `change_log` table; prints additions | Yi-Chen frustration: no RSS/digest; brief Top Workflow #2 | Yi-Chen, Mei-Ling |
| 2 | Saved-query daily digest | `watch query <terms> --since YYYY-MM-DD` | 8/10 | Stored named query in local `watchlist` table; runs same FJUD search; prints JIDs newer than cursor | Brief Top Workflow #2; weekly ritual for 3 personas | Mei-Ling, Yi-Chen, Wu |
| 3 | Statute-citation graph | `cites statute <code> [--article N]` | 9/10 | Local query over `judgments` × extracted `citations` table joined by court/year | Brief Build Priority 7; competitor gap (only Lawsnote does cross-judgment citation queries) | Mei-Ling, Wu |
| 4 | Reverse-citation precedent lookup | `cited-by <jid>` | 8/10 | Reverse index of citations table | Brief Top Workflow #4; explicit paralegal use | Mei-Ling, Wu |
| 5 | Sentencing distribution | `sentences --statute <code>` | 7/10 | Regex extraction of 主文 patterns at sync into `sentences` table; aggregation prints histogram | Brief Build Priority 7; Wu's ritual; samttoo22 has no analytics | Wu, Mei-Ling |
| 6 | Case-character (字別) catalog | `case-types list` | 6/10 | Aggregation of `case_character` grouped by court | Brief mentions 字別 as core taxonomy; agent-friendly enum discovery | Mei-Ling, agent |
| 7 | Appeal-chain walker | `appeal-chain <jid>` | 7/10 | Joins court-hierarchy + 案號 root match in local store | Brief Top Workflow #4; paralegal explicit | Mei-Ling |
| 8 | Privacy-purge sweeper | `purge --orphans` | 7/10 | Re-fetches synced JIDs via JDoc; on `查無資料` deletes row + audit-log | Brief Reachability §3: ToS-required | Wu, Mei-Ling, all |
| 9 | Knowledge ↔ judgment linker | `knowledge link <par>` | 6/10 | FJUDKM commentary → extract statute refs → match local `citations` | Only meaningful join across the two sources | Wu, Mei-Ling |
| 10 | Related-case discovery | `related <jid>` | 7/10 | Jaccard similarity over citation set, filtered to same court tier and ±2 years | Brief Build Priority 7; agent and paralegal want "more like this" | Mei-Ling, agent |
| 11 | Service-window reporter | `doctor window` | 5/10 | Compares Taipei time to 0–6 AM API window; prints status + seconds-to-window | Brief Reachability calls out the window; Mei-Ling and agent hit it | All, especially agent |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C6 Court-vs-court comparison | Thin wrapper — same data is one `sql` query away once C5 ships | C5 Sentencing distribution |
| C9 民國 date helper command | Demoted to date-flag parser shared by every command | (folded into flag handling) |
| C12 PDF + text bundler | Wrapper — overlaps absorbed feature #8 | Absorbed feature #8 |
| C14 Judge productivity | Verifiability fails on judge-name extraction; weekly use below bar | C5 Sentencing distribution |
| C16 Constitutional-court tracker | Below weekly bar; subsumed by C2 with `--type constitutional` | C2 Saved-query daily digest |

## Notes on `doctor window` (#11)

The brief said the 0–6 AM window applies to the *official* open data API. After Phase 1.6 the user chose to skip the official API entirely. `doctor window` should still ship — it doubles as a useful "is this a good time to bulk-sync the website" hint (best practice during off-peak hours), and it warns operators who later flip on official-API support. Score remains 5/10.

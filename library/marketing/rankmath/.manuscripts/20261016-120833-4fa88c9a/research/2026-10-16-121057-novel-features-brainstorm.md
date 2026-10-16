## Customer model
- Maya, owner-operator of two WordPress content sites (example.com, example.org). Today: wp-admin per site + Rank Math Analytics + GSC + spreadsheet; view-source per URL to see what Google sees. Ritual: Monday dashboard check, edit titles/descriptions, push URLs to IndexNow. Frustration: no site-wide list of missing/duplicate title/description or accidental noindex.
- Dev, agency SEO lead with ~12 client WP sites. Today: one browser profile per client; competitor MCPs need bridge plugins and lack analytics/indexing. Ritual: weekly health sweep per client. Frustration: no cross-site view; Pro/free module variance breaks scripts.
- Content-ops agent (Claude/Codex). Today: writes blind via updateMeta or ad-hoc scraping. Ritual: apply bulk meta edits, verify, request indexing. Frustration: no structured "what's wrong now" list; can't prove fix landed or index only changed URLs.

## Candidates (pre-cut)
1 audit (a)(c) keep · 2 drift (b) sibling · 3 schema-coverage (b) sibling · 4 head snapshot (c) infra · 5 meta plan (a) already absorbed · 6 opportunities (b)(c) keep · 7 coverage (b)(c) keep · 8 reindex (a)(b) keep · 9 portfolio (a) keep · 10 decay (b) reimplementation · 11 cannibalization (b) data gap · 12 redirect-chains (b) no list route · 13 meta fill (a) LLM dependency · 14 orphans (b) reimplements Link Genius · 15 meta rollback (a) data-loss risk · 16 track-suggest (b) Pro-gated/speculative

## Survivors and kills
### Survivors
| # | Feature | Command | Score | Buildability | How It Works | Long Description | Persona |
|---|---|---|---|---|---|---|---|
| 1 | SEO head audit | audit | 10/10 | hand-code | wp/v2 inventory + rendered-head parse (getHead when enabled) stored as seo_snapshots; missing/duplicate title+description, missing canonical, noindex, missing JSON-LD, change since previous snapshot | Use this command to audit on-page SEO tags across all synced posts. Do NOT use this command for internal-link audits; use 'link-genius' audit commands instead. Do NOT use it for Google index status; use 'coverage' instead. | Maya, Agent |
| 2 | Striking-distance opportunities | opportunities | 7/10 | hand-code | GSC keyword/post rows joined to seo_snapshots: queries at position 4-20 whose ranking post's title/description lacks the query | none | Maya, Agent |
| 3 | Index coverage gaps | coverage | 8/10 | hand-code | inventory + an/inspectionResults + snapshot robots: published URLs not indexed / never inspected, excluding intentional noindex | Use this command to find published posts Google has not indexed. Do NOT use this command to push recently edited URLs; use 'reindex' instead. | Maya, Dev |
| 4 | Reindex changed posts | reindex | 8/10 | hand-code | wp/v2 modified_after minus in/getLog → in/submitUrls; dry-run default, --confirm to send | Use this command to submit recently modified posts to IndexNow. Do NOT use this command to find unindexed posts; use 'coverage' instead. | Maya, Agent |
| 5 | Portfolio rollup | portfolio | 7/10 | hand-code | an/analyticsSummary across every configured site profile, skipping sites lacking the module | none | Dev |
### Killed candidates
| Feature | Kill reason | Closest sibling |
|---|---|---|
| drift | folded into audit output | audit |
| schema-coverage | folded into audit --check schema | audit |
| head snapshot | infrastructure | audit |
| meta plan | already absorbed (meta import) | audit |
| decay | Rank Math postsRows already return period diffs | portfolio |
| cannibalization | focus keyword not readable | opportunities |
| redirect-chains | no redirection list route | audit |
| meta fill | LLM dependency | audit |
| orphans | reimplements Link Genius, Pro-gated | opportunities |
| meta rollback | rendered titles are expanded templates; restore would corrupt | audit |
| track-suggest | Pro-gated, speculative | opportunities |

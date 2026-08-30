# Webflow CLI — Novel Features Brainstorm (Step 1.5c.5 subagent audit trail)

Run: 20260728-073020-15799c7c · First print (no prior research) · Spawned once, general-purpose subagent.

## Customer model

### Priya — delivery lead at a 6-person Webflow agency, 14 live client sites

**Today (without this CLI):** Priya keeps 14 Webflow dashboard tabs pinned, one per client. Before any client launch she walks a manual checklist by hand: open Site settings → check the redirect table, open the SEO tab page by page, open Assets to see if the old brand images are still there, open Forms to confirm submissions are still routing. The official `@webflow/webflow-cli` gives her `sites list` and `sites publish` and nothing else — it has no pages surface at all, so every page-level check is a click. She cannot answer "across all 14 clients, which sites have unpublished changes sitting in staging right now?" without opening 14 tabs.

**Weekly ritual:** A Monday sweep across every client site to find what drifted over the weekend — content the client edited in the Designer but never published, redirects that broke when someone renamed a page, assets nobody references anymore — and a Thursday/Friday pre-launch hygiene pass on whichever site is shipping.

**Frustration:** There is no cross-site view. Every question that starts with "across all my clients…" costs 14 manual repetitions, and the `webflow-skills` audit skills re-fetch everything from the API each time they run, which at 60 requests/minute on a Starter-plan client is slower than doing it by hand.

### Marcus — content ops manager for one SaaS marketing site, 400+ CMS blog items

**Today (without this CLI):** Marcus lives inside one collection. He uses `webflow-api` from a Node script for anything bulk, because the official CLI's `cms items` commands are one-at-a-time and give him no way to select items by a condition. When he needs to retag 60 posts or flip an author field, he writes a throwaway script that pages the API, filters in JS, and PATCHes in a loop — and it dies on 429 halfway through because he forgot to pace it. He cannot answer "which of my 400 items were edited in staging but never published?" at all; the API gives him a staged list and a live list and no comparison.

**Weekly ritual:** A publish cycle. Draft/edit a batch of CMS items in staging, spot-check them, then publish the batch and the site. Plus a recurring bulk edit — retag, reassign, fix a field the marketing team changed its mind about.

**Frustration:** The staged-versus-live distinction is the most important fact in the whole API and there is no tool that shows him the difference. He publishes on faith, then finds out a week later that three posts went live with an empty meta description because they were never actually staged the way he thought.

### Nadia — freelance SEO consultant retained by three Webflow clients

**Today (without this CLI):** Nadia's entire deliverable is a spreadsheet. She uses Screaming Frog to crawl the rendered site, then manually cross-references the crawl against Webflow's page settings, because the crawler sees the rendered output but not `seo.title`, `seo.description`, `openGraph`, `isDraft`, or the redirect table that Webflow actually stores. No CLI exposes any of that — not the official one, not `webflowctl`. When she finds a problem she has to walk the client through fixing it in the Designer, page by page, because she has no write path either.

**Weekly ritual:** A metadata audit pass on one client site — find pages with missing titles, descriptions over the SERP truncation length, duplicated titles across pages, missing openGraph images, and draft pages that were meant to ship — then hand back a prioritized fix list.

**Frustration:** The audit is a re-crawl every single time. There is no stored snapshot, so she cannot say "here is what changed since my last audit," and building the missing/duplicate/over-length report is a manual pivot table over a CSV she assembled by hand.

### Ari — automation engineer running a headless content agent in CI

**Today (without this CLI):** Ari's pipeline needs to read and rewrite page copy programmatically. The official MCP server has `data_pages_tool`, but the Designer-bridge tools (`deElement`, `dePages`) require the Webflow Designer open with a companion app running, which is impossible in CI. So the agent talks to the raw Data API through `webflow-api`, and every "what should I change?" decision costs a fresh round of API calls because there is no data layer to query. The `webflow-skills` collection is prompt guidance calling the MCP — no storage, so every comparison is single-shot.

**Weekly ritual:** Nightly CI runs that inspect a site's content state, decide on a batch of edits, apply them, and report a diff back into a PR comment.

**Frustration:** The agent burns its 60-requests-per-minute budget re-reading state it already read an hour ago, and gets prose-shaped output it has to re-parse instead of a machine-shaped answer to "what is wrong with this site right now."

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Data source | Long Description |
|---|------|---------|-------------|---------|--------|-------------|------------------|
| A1 | Page SEO audit | `seo audit <site-id>` | Scores every synced page's `seo.title` / `seo.description` / `openGraph` / `slug` / `isDraft` for missing, duplicated, and over-length values. | Nadia | (a) persona, (e) user vision | local | planned |
| A2 | Staged-vs-live drift | `drift <collection-id>` | Field-by-field comparison of synced staged items against synced live items. | Marcus | (b) staged/live duality | local | planned |
| A3 | Publish preview | `publish preview <site-id>` | Everything that would change if you published this site now. | Priya, Ari | (b) + (e) | local | planned |
| A4 | Query-driven bulk field set | `items bulk-set <collection-id>` | Selects items from the local mirror by `--match` pairs, PATCHes them upstream with `Retry-After`-aware pacing. | Marcus | (e) user vision | see note | planned |
| A5 | Redirect table audit | `redirects audit <site-id>` | Joins synced redirects against page and item slugs. | Priya, Nadia | (c) cross-entity join | local | planned |
| A6 | Collection field completeness | `collections completeness <collection-id>` | Per-field fill rate across every synced item. | Marcus | (c) cross-entity join | local | planned |
| A7 | Multi-site rollup | `overview` | One row per synced site. | Priya | (a) persona | local | planned |
| A8 | Orphan asset finder | `assets orphans <site-id>` | Assets with zero references in synced item field values. | Priya | (c) | local | none |
| A9 | Internal link checker | `links check <site-id>` | Extracts hrefs from page DOM, flags unknown slugs. | Nadia | (b) DOM tree | live | none |
| A10 | Page copy roundtrip | `pages copy pull/push <page-id>` | Flattens page DOM to an editable text file and back. | Ari | (b) + (e) | live | none |
| A11 | Duplicate metadata detector | `seo duplicates <site-id>` | Pages/items sharing identical title, description, or slug. | Nadia | (c) | local | none |
| A12 | Form submission digest | `forms digest <site-id>` | Groups synced submissions by form over a window. | Priya | (a) | local | none |
| A13 | Rate-limit budget planner | `budget <site-id>` | Estimates API calls for a full sync against the plan ceiling. | Ari | (b) tier split | computed | none |
| A14 | Custom code inventory | `custom-code audit <site-id>` | Site + per-page scripts flagged by external host and duplication. | Priya | (b) | live | none |
| A15 | Stale content report | `stale <site-id>` | Pages and items not updated within a window. | Nadia | (c) | local | none |
| A16 | Low-stock SKU report | `inventory low-stock <site-id>` | SKUs below an inventory threshold. | none | (b) ecommerce | local | none |

Inline kill/keep checks applied during generation:

- **LLM dependency** — A1 was reframed before entering the list. "Score page SEO quality" would need prose judgment and is a kill; what is listed is purely mechanical (missing value, duplicate value, character-length threshold, boolean `isDraft`), with `--json` output so the user can pipe it into an LLM themselves.
- **External service** — no candidate calls anything outside the Webflow Data API.
- **Auth gap** — A16 requires an ecommerce plan and `ecommerce:read` scope no named persona has. A4 requires `cms:write`, the same scope the absorbed items CRUD already needs.
- **Scope creep** — A10 is a file-sync application, not a command. A14 needs one API call per page, hundreds of sequential calls on a 60/min plan.
- **Verifiability** — A9 and A14 cannot be verified in dogfood without a populated live site.
- **Reimplementation** — none of the local candidates fake an API response.

## Survivors and kills

### Pass 3 forced answers

**A1 `seo audit`** — 1. Weekly: yes, this *is* Nadia's weekly deliverable. 2. Wrapper: no; the API has no audit endpoint. 3. Transcendence: local SQLite over the synced `pages` table plus a self-join for duplicates. 4. Sibling kill: A11 `seo duplicates`, folded in. 5. Buildability: `hand-code`. 6. Long-description validity: names `publish preview` and `collections completeness`, both survivors.

**A2 `drift`** — 1. Weekly: yes, once per publish cycle. 2. Wrapper: no; two endpoint families exist and nothing compares them. 3. Transcendence: local join of staged against live items. 4. Sibling kill: A15 `stale`, a weaker timestamp-based version of the same question. 5. Buildability: `hand-code`. 6. Long-description validity: names `publish preview`, a survivor.

**A3 `publish preview`** — 1. Weekly: yes; before every client publish and nightly in CI. 2. Wrapper: no; the publish endpoint does not describe pending changes. 3. Transcendence: cross-table join of `sites.lastPublished` against `pages.lastUpdated`, item counts, redirects. 4. Sibling kill: A13 `budget`. 5. Buildability: `hand-code`. 6. Long-description validity: names `drift` and `overview`, both survivors.

**A4 `items bulk-set`** — 1. Weekly: yes; the recurring bulk edit is half of Marcus's job. 2. Wrapper: no; `POST /items/bulk` is create-only, so there is no bulk-PATCH endpoint to wrap. 3. Transcendence: local selection feeding a paced write loop keyed to `Retry-After`. 4. Sibling kill: A10 `pages copy`. 5. Buildability: `hand-code`. 6. Long-description validity: names `publish preview` (survivor) and `items publish` (absorbed row 13).

**A5 `redirects audit`** — 1. Weekly: yes for the Monday sweep. 2. Wrapper: no; CRUD is absorbed row 32, this is a validation join. 3. Transcendence: cross-entity join of redirects against page and item slugs. 4. Sibling kill: A9 `links check`. 5. Buildability: `hand-code`. 6. Long-description validity: names `seo audit`, a survivor.

**A6 `collections completeness`** — 1. Weekly: yes; before every content batch. 2. Wrapper: no; nothing computes coverage across schema and values. 3. Transcendence: local join of fields schema against item values. 4. Sibling kill: A8 `assets orphans`. 5. Buildability: `hand-code`. 6. Long-description validity: names `seo audit`, a survivor.

**A7 `overview`** — 1. Weekly: yes, this is Priya's Monday morning. 2. Wrapper: no; there is no cross-site aggregate endpoint. 3. Transcendence: multi-table local rollup across every synced site. 4. Sibling kill: A12 `forms digest`. 5. Buildability: `hand-code`. 6. Long-description validity: names `publish preview` and `seo audit`, both survivors.

### Survivors

See the absorb manifest's Transcendence table for the shipping form of these seven rows.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| A8 Orphan asset finder (`assets orphans`) | Page DOM node trees are not in the synced entity list, so an asset referenced only from page markup would be a false-positive orphan on a destructive-looking finding. | `collections completeness` |
| A9 Internal link checker (`links check`) | Requires page DOM in the local store, which `sync` does not populate; the live fallback is one `GET /pages/{id}/dom` per page, blowing the 60 req/min ceiling. | `redirects audit` |
| A11 Duplicate metadata detector (`seo duplicates`) | A strict subset of `seo audit`'s duplicate check; two commands for one question. | `seo audit` |
| A10 Page copy roundtrip (`pages copy pull/push`) | Thin file plumbing over the already-absorbed page DOM read/write; the roundtrip file format is an application, not a command. | `items bulk-set` |
| A12 Form submission digest (`forms digest`) | Thin over the absorbed submissions list plus absorbed offline search; adds grouping and nothing else. | `overview` |
| A13 Rate-limit budget planner (`budget`) | Run monthly at best, and the absorbed `doctor` already reports the plan's rate-limit tier. | `publish preview` |
| A14 Custom code inventory (`custom-code audit`) | One API call per page with no synced table to read from — hundreds of sequential calls against a 60 req/min ceiling. | `redirects audit` |
| A15 Stale content report (`stale`) | Answers with timestamps the same question `drift` answers with actual field values. | `drift` |
| A16 Low-stock SKU report (`inventory low-stock`) | No named persona has an ecommerce plan or the `ecommerce:read` scope. | `collections completeness` |

## Build-phase notes carried forward

- Every survivor except `items bulk-set` declares `// pp:data-source local`.
- `items bulk-set`: the subagent proposed `computed`. **Overridden to `local`** — the directive describes where the command's *data* comes from, and bulk-set's selection comes from the local mirror; the upstream PATCH is the action, not the data source. It rejects `--data-source live` with a "no live equivalent — run sync first" error. `computed` is reserved for pure policy/math commands and would be dishonest here.
- Every local-store survivor calls `if !hintIfUnsynced(cmd, db, "<resource>") { hintIfStale(cmd, db, "<resource>", flags.maxAge) }` before returning results.
- All joins use the drain-first pattern: scan the parent result set into structs, check `rows.Err()`, close `rows`, then run follow-up queries. No nested `QueryContext` on an open `*sql.Rows`.
- `items bulk-set` must not call `store.Upsert` / `store.UpsertBatch` inside an open `db.DB().BeginTx` write transaction.
- `items bulk-set` defaults to dry-run so dogfood never mutates; `--apply` executes.

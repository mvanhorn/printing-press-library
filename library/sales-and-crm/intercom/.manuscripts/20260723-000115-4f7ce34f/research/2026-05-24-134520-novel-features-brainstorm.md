# Novel Features Brainstorm — intercom-pp-cli

## Customer model

### Mei — Support Operations Lead at a mid-market B2B SaaS (EU workspace)
- **Today:** Runs a 14-person support team on Intercom EU. Lives in the Intercom web UI's Inbox view; exports CSVs weekly for her ops dashboard. The official Intercom MCP is US-only and useless to her workspace.
- **Weekly ritual:** Every Monday she pulls last week's conversation volume by team, by tag, by first-response time — manually exported from the UI and pivoted in Sheets. During incidents she opens 40 browser tabs to manually tag affected conversations.
- **Frustration:** UI is fast for one conversation, brutal for fifty. Wants `intercom-pp-cli conversations bulk-tag --filter '…' --tag incident-2026-05-24` and the loop closed.

### Devon — Data Engineer at a Series-C ecommerce brand
- **Today:** Owns the Intercom→Snowflake pipeline. Custom Python job using `python-intercom`, runs nightly on Airflow. Breaks any time Intercom adds a custom attribute. Data Export API requires job-polling boilerplate he keeps reinventing.
- **Weekly ritual:** Sunday-night DAG; Monday eyeball row counts; reconcile drift. 2-3 hours/week on Intercom-side debugging — usually a merged contact or soft-deleted record.
- **Frustration:** Wants `sync` + `sql` + `analytics` so Mei can answer her own questions without round-tripping through him. Wants a local file he can diff between syncs so drift is observable.

### Priya — Docs / Knowledge Manager at a developer-tools startup
- **Today:** Owns 320 help-center articles across 4 languages. Engineers want them in git for changelog cross-references. Uses `kaosensei/intercom-mcp` for ad-hoc edits but it can't do the export-to-git round-trip.
- **Weekly ritual:** Sprint-end: review which articles changed, update translations, archive stale ones. Manually opens each article, copies HTML, pastes into a doc, sends to translators, pastes back. Half a day every other Friday.
- **Frustration:** Wants `articles pull --to ./articles/` to flatten everything to versioned markdown, edit locally, then `articles push` to sync back — with multilingual handled as sibling files.

### Ash — AI/Platform engineer building a Fin-assisted support agent
- **Today:** Internal Claude agent triaging incoming Intercom conversations. EU workspace, so the official MCP is unusable. ~600 lines of TypeScript glue today.
- **Weekly ritual:** Iterates on prompts, replays last week's conversations through her agent in eval mode, checks where it would have escalated wrongly.
- **Frustration:** Wants every Intercom action surfaced as an MCP tool with read-only/destructive hints set correctly, regional base URL handling, and a `--filter` AST she can build programmatically instead of templating Intercom's nested predicate JSON by hand.

## Candidates (pre-cut)

C1 filter-piped bulk-tag · C2 incident-tag · C3 articles pull/push · C4 SLA analytics · C5 contact dupes · C6 region-aware login · C7 filter explain · C8 conversation thread render · C9 stale conversations · C10 Data Export with wait · C11 workspace inventory · C12 article translation gap · C13 conversation rebalance · C14 contact upsert from JSONL · C15 rate-budget meter · C16 Fin AI replay · C17 tag sprawl audit · C18 contact 360

(See per-candidate analysis below in the cut.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Incident-tag dry-run | `conversations incident-tag --mentions "<phrase>" --since 24h --tag <slug> [--apply]` | 9/10 | hand-code | Calls `/conversations/search` with body-substring + updated_at predicate, prints would-tag diff; on `--apply`, fans out tag mutations through `cliutil.AdaptiveLimiter` respecting `X-RateLimit-Remaining` | Brief Top Workflow #3; kyoji2 and 44-pixels MCPs both surface tag-conversation; no existing CLI offers filter-piped + default-dry-run shape |
| 2 | Articles git round-trip | `articles pull --to ./articles/`, `articles push` | 9/10 | hand-code | Paginates `/articles` + `/help_center/collections`, writes `<id>-<slug>.<lang>.md` with YAML frontmatter; push diffs against last-pull manifest and PATCHes changes per locale | Brief Top Workflow #4 + Priya persona; `kaosensei/intercom-mcp` exists because article CRUD is painful via UI; no tool round-trips to markdown |
| 3 | SLA analytics | `conversations sla --group-by team --metric first-response,resolution` | 8/10 | hand-code | SQL over local `conversations` join `conversation_parts` (min admin-author `created_at` minus conversation `created_at`) join `teams`; emits JSON/CSV/table | Brief Top Workflow #2 + Mei's Monday ritual; warehouse teams currently replicate to Snowflake to answer this |
| 4 | Contact dupe clusters | `contacts dupes --by email\|external_id\|phone [--all]` | 7/10 | hand-code | SQL over local `contacts` GROUP BY normalized key, returns cluster JSONL with member IDs sorted by `updated_at`; suggests winner per `--strategy newest\|oldest` | GetintheLoop/intercom-cli ships "extract duplicate contacts" as headline; Devon persona; merged-contact handling in Intercom search confirms the problem |
| 5 | Filter explain | `filter explain '<expression>' [--api conversations\|contacts]` | 7/10 | hand-code | Tokenizes a human-shaped boolean expression (`role:user AND custom.plan="pro"`), emits the Intercom nested predicate JSON, no API call | Brief Codebase Intelligence calls out the search-AST split; intercom-node SDK issues mention predicate-shape confusion |
| 6 | Stale conversations | `conversations stale --idle 7d --state open [--assigned-to <admin>]` | 7/10 | hand-code | SQL over local `conversations` where `state=open` AND `max(parts.created_at) < now - <duration>`; optional assignee filter | Brief Top Workflow #1; Mei persona; existing MCPs offer search-by-state but not idle-time |
| 7 | Article translation gap | `articles translations [--locale fr,de]` | 6/10 | hand-code | SQL over local `articles` grouping by `parent_id`, reports which `default_locale` articles lack translations in each enabled locale | Brief Top Workflow #4 + Priya persona; multilingual CRUD is absorbed but the gap report is novel |
| 8 | Rate-budget meter | `budget` | 6/10 | hand-code | Issues a single cheap GET (`/me`), reads `X-RateLimit-Remaining` and `X-RateLimit-Reset` from response headers, prints current 10s-window headroom + ETA | Brief Codebase Intelligence calls out the 1000 req/10s app limit + headers; intercom-node SDK rate-limit issue exists |
| 9 | Contact 360 | `contact 360 <email\|external_id\|id>` | 8/10 | hand-code | Resolves contact, then SQL-joins synced `companies` (via attached_companies), `conversations` (author=contact), `tickets` (contact attached), `notes`, `tags`; single nested JSON payload | Brief Data Layer enumerates all joined entities; cross-entity 360 is uniquely valuable to triage (Mei) and agents (Ash) |

### Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| C1 filter-piped bulk-tag | Sibling overlap with absorbed file-driven bulk-tag (#17); collapse to a `--filter` flag on the existing command | Manifest #17 file-driven bulk operations |
| C6 region-aware login | Already absorbed (manifest #13) | Manifest #13 `--region` flag |
| C10 Data Export with wait | Already absorbed (manifest #21) | Manifest #21 `dataexport create --wait` |
| C13 conversation rebalance | Sibling exists in spec; high blast radius; hard to verify | Endpoint mirror `conversations runAssignmentRules` |
| C14 contact upsert from JSONL | Already absorbed (manifest #19) | Manifest #19 `apply` reads JSONL |
| C16 Fin AI replay | Fin endpoints stream via webhook only; not callable from a CLI | None — webhook-only feature outside CLI surface |
| C8 conversation thread render | Leverage too thin once `--select` and `--agent` JSON exist; subjective formatting | `conversations show <id>` + `--select`/`--agent` |
| C11 workspace inventory | Fails Weekly Use; fans out to existing list commands without adding compute | Per-resource list endpoints |
| C17 tag sprawl audit | Fails Weekly Use — quarterly hygiene cadence; duplicate-detection overlap with C5 | C5 contact dupes + `tags list` |

# Novel Features Brainstorm — gohighlevel-pp-cli

## Customer model

**Jen — Director of Operations & Technology, KW Capital Properties (multi-office KW brokerage, DC/MD/VA)**

*Today (without this CLI):* Maintains 30+ Python scripts in `~/Documents/KWCP-Scripts/` that hit GHL through `safe_get`/`safe_post` helpers in `kwcp_config.py`. Pulls contacts via `/contacts/search` with hand-rolled pagination, looks up custom field IDs in a `CLAUDE.md` cheat sheet (`Agent Affiliation = ilvReXHcDuxetPOZ4wCK`), runs nightly orchestrator at 3am, exports CSVs to OneDrive for the team. Lives in iTerm with `jq`, switches between KWCP and THINK location IDs by hand-editing config.

*Weekly ritual:* Sunday-night L10 prep (`kwcp_sunday.py`) runs lead-followup → dup-triage → hot-followup. Nightly dashboard refresh, Monday recruit list, daily bulk tag operations after every training event (300-500 contacts). Custom field name→ID lookups happen 20+ times a day across scripts and ad-hoc terminal pokes.

*Frustration:* Three things kill her week. (1) The connection-drop bug — every fresh script discovers `requests.get` fails and has to be re-routed through `safe_get`. (2) Hardcoded custom field IDs that go stale every time GHL changes something. (3) No way to ask "which opportunities have been sitting in Recruit for 14+ days" without a custom script — GHL doesn't expose stage history, and her `kwcp_growth_metrics.py` reinvents this every week.

**Kymber — CEO, primary reviewer of strategic reports**

*Today:* Reads weekly CSVs Jen drops into OneDrive. Asks ad-hoc questions in Slack that turn into 30-minute Python sessions for Jen.

*Weekly ritual:* Monday L10 — funnel stage counts, stale opps, hot-recruit list. Quarterly OKRs.

*Frustration:* The lag. Jen's bandwidth becomes the bottleneck for "just one more cut of the data."

**Claude Code agent (Jen's automation copilot)**

*Today:* Calls the local `ghl-mcp-server-kwcp` MCP. MCP has no local cache, so every bulk question hits the live API and dies on the connection drop.

*Weekly ritual:* Drives Jen's nightly scripts, drafts automations, answers SQL-shaped questions hitting a non-SQL API.

*Frustration:* Round-trips to the live API for bulk reads. Every join becomes 4 paginated MCP calls.

## Candidates (pre-cut)

16 candidates generated, see live subagent output. Sources covered:
- (a) Persona-driven: C1, C2, C4, C6, C7, C8
- (b) Service-specific: C3, C9, C10, C11, C12, C15
- (c) Cross-entity: C5, C7, C8, C16
- (e) User briefing: C1, C2, C3, C4, C6, C8, C9

Borderline candidates pre-cut at this stage: C14 (TSV flag), C13 (subsumed by doctor).

## Survivors and kills

### Survivors

| # | Feature | Command | Buildability | Score | Persona Served | Buildability Proof |
|---|---------|---------|--------------|-------|----------------|--------------------|
| 1 | Stale opportunity report | `ghlcli opp stale --pipeline <name> --stage <name> --days N [--include-history]` | hand-code | 9/10 | Jen, Kymber | Local SQLite `stage_transitions` table built from sync diffs (GHL exposes no stage-history endpoint); joins opportunities × stages × pipelines and filters by `entered_stage_at < now() - N days`. |
| 2 | Pipeline funnel snapshot | `ghlcli opp funnel --pipeline <name> [--tsv\|--json]` | hand-code | 9/10 | Jen, Kymber | Aggregates `opportunities` grouped by `stage_id` joined to `pipelines` + `stages` in local SQLite; emits Looker-friendly TSV column order. No GHL funnel endpoint. |
| 3 | Custom field name resolver | `ghlcli field id <name>` plus universal `--custom-field "Name=Value"` interceptor | hand-code | 10/10 | Jen, Claude | Local SQLite `custom_fields` table populated by `/locations/{id}/customFields`; flag interceptor translates `"Agent Affiliation=KWCP"` → `ilvReXHcDuxetPOZ4wCK=KWCP` before every API call. Did-you-mean via Levenshtein. |
| 4 | Bulk tag from stdin | `ghlcli contact bulk-tag --tag <t> [--remove] [--dry-run]` | hand-code | 9/10 | Jen | Reads emails/IDs from stdin, looks up contact IDs from local cache, calls `POST /contacts/{id}/tags` in batches of 100 with exponential backoff. The leverage is the chunking + dedup, not the endpoint call. |
| 5 | SQL-on-cache | `ghlcli sql "<query>"` | hand-code | 10/10 | Jen, Claude | Opens local SQLite cache populated by `ghlcli sync`; exposes contacts/opportunities/pipelines/stages/tags/custom_fields/conversations/messages/appointments as tables. Read-only. |
| 6 | Dedup with richness scoring | `ghlcli contact dedup --by email,phone --dry-run [--apply]` | hand-code | 8/10 | Jen | Local SQLite groups by lowercased email + E.164 phone, scores each by filled-field count + `dateUpdated` recency, emits merge plan JSON; `--apply` calls `POST /contacts/upsert`. |
| 7 | Engagement decay alert | `ghlcli contact decay --stage <name> --idle-days N` | hand-code | 8/10 | Jen, Kymber | Joins `opportunities` × `contacts` × `conversations` in local SQLite; flags rows where `max(message.dateAdded)` is older than N days. |
| 8 | Hot follow-up scorecard | `ghlcli recruit hot --threshold N` | hand-code | 7/10 | Jen, Kymber | Composite scoring formula from user memory `kwcp_hot_followup_scoring_v2.md`: production + engagement + recruit tags via SQL over local cache. |
| 9 | Multi-location config + flag | `ghlcli config use <name>`; `--location <name>` global | hand-code | 8/10 | Jen | Named profiles in `~/.config/ghlcli/config.toml` (KWCP, THINK); every command resolves `locationId` from profile so cross-tenant calls can't leak. |
| 10 | Doctor | `ghlcli doctor` | hand-code | 7/10 | Jen, Claude | Validates `GHL_PIT_TOKEN` prefix (auto-lowercases), pings `/locations/{id}`, reports cache freshness per table from `sync_state`, warns on stale workflow membership. |
| 11 | Conversation thread reconstruction | `ghlcli convo thread --contact <email\|id>` | hand-code | 7/10 | Jen, Claude | Resolves contact ID, queries local SQLite `messages` table, sorts by `dateAdded` across channels, emits unified timeline with `channel`, `direction`, `from`, `body`, `timestamp`. |

### Killed candidates

- **C12 Stage transition log** — folded into C1 as `--include-history` flag.
- **C13 Workflow enroll diagnostic** — folded into C10 doctor as a check.
- **C14 TSV output** — not a feature, a flag.
- **C15 Watched search (--watch)** — defer; cohort SQL covers the use case for v1.
- **C16 Tag explorer** — expressible as one-line SQL against C5.

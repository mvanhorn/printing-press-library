# Plaud CLI — Absorb Manifest

> Phase 1.5 manifest for `plaud-pp-cli`. Every feature here is shipping scope unless explicitly marked `(stub)`. Built via novel-features subagent + parallel deep-research across 30+ community tools.

## Absorbed features (match or beat everything that exists)

These are the table-stakes features that any Plaud CLI must have. Every row IS shipping scope.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | Email+password login → JWT | sergivalverde/plaud-toolkit | `auth login` → `POST /auth/access-token` (form-urlencoded), persist to `~/.plaud-pp-cli/config.json` (mode 0600) | Auto re-login 30d before exp; region routing on `-302` retry |
| 2 | Chrome JWT extraction | applaud | `auth login --chrome` → read `PLADU_bearer` from Chrome/Brave/Arc/Vivaldi LevelDB | No-password mode; multi-browser fallback |
| 3 | Logout | official CLI | `auth logout` → clears local config | — |
| 4 | Current user info | official CLI, official MCP, sergivalverde | `users me` | Cached after sync; `--no-cache` for live |
| 5 | List recordings | official CLI `files`, applaud, every tool | `recordings list` | Local store after sync; `--json --select`; `--limit/offset`; `--data-source auto|local|live` |
| 6 | List by date range | official CLI `recent --days N` | `recordings list --since 30d --until 7d` | Date math (relative + ISO); aliases (`recent`, `today`) |
| 7 | Recordings today | official CLI `today` | `recordings today` | Local-store backed |
| 8 | Recording detail | official CLI `file <id>`, official MCP `get_file` | `recordings get <id>` | Includes `thought_partner`, `content_list`, `embeddings` |
| 9 | Transcript | official CLI `transcript`, official MCP `get_transcript` | `recordings transcript <id>` | Speaker-diarized segments; `--format text|json|srt|vtt|markdown` |
| 10 | Summary | official CLI `summary`, official MCP `get_note` | `recordings summary <id>` | Normalizes the 4 inconsistent shapes from `/ai/transsumm` |
| 11 | Audio temp URL | official CLI `audio`, sergivalverde `plaud_get_mp3_url` | `recordings audio-url <id>` | `--opus` flag for OGG |
| 12 | Audio download | applaud, Plaud_BulkDownloader, iiAtlas | `recordings download <id> --out file.mp3` | Streaming; `--opus`; resumable |
| 13 | Document export | Plaud_BulkDownloader, applaud | `recordings export <id> --format docx\|pdf\|txt\|md --kind transcript\|summary` | Wraps `/file/document/export` with telemetry pre-flight |
| 14 | Bulk download | iiAtlas, Plaud_BulkDownloader | `recordings download --all --since 30d --concurrency 4` | Resumable; filename templating; safe naming |
| 15 | Keyword search | official CLI `search` (client-side only) | `search "<query>"` | FTS5 backed (orders of magnitude faster); `--phrase`, `--regex` modes |
| 16 | Search by date | official CLI `search --from --to` | `search --since/--until` | Relative date math |
| 17 | List file tags | Plaud_API `GetFileTagsAsync` | `tags list` | Tree view; `--json` |
| 18 | List recordings in tag | applaud | `tags recordings <tag>` | Local-store backed |
| 19 | Trash recording(s) | Plaud_API `TrashRecordingsAsync` | `recordings trash <id>...` | Batch; `--dry-run`; idempotent |
| 20 | Restore from trash | Plaud_API `UnTrashRecordingsAsync` | `recordings untrash <id>...` | Batch; `--dry-run` |
| 21 | Permanent delete | Plaud_API `PermanentlyDeleteRecordingsAsync` | `recordings delete <id>... --confirm` | Requires `--confirm`; idempotent |
| 22 | List trash | derived (`is_trash=1`) | `recordings list --trash` | — |
| 23 | Create shareable link | Plaud_API `CreateShareableLinkAsync` | `recordings share <id>` | Permission flags |
| 24 | Sync (incremental) | applaud, leonardsellem, sergivalverde | `sync` (delta via `edit_time`) | All-entities; resumable; `--dry-run` |
| 25 | Sync (full) | leonardsellem | `sync --full` | Walks all pages; safe re-run |
| 26 | Speaker-diarized output | applaud, leonardsellem, xclgordon | All transcript commands surface `speaker` | First-class column; FTS5 joinable |
| 27 | Region routing | sergivalverde (us/eu only) | `--region us\|eu\|ap` override + auto-detect on `-302` retry | All three regions (vs sergivalverde's two) |
| 28 | MCP server | official MCP, sergivalverde, charathram, PlaudBlender | Runtime walker mirrors every CLI command as MCP tool | One surface for CLI + MCP; built-in `search`, `sql`, `context` agent tools |
| 29 | Obsidian-shape export | leonardsellem | `export obsidian --out ./vault` | YAML frontmatter with stable `file_id`; transcript+summary+highlights+filetags |
| 30 | Doctor / health check | every CLI | `doctor` | Verifies JWT, region, `/user/me`, days-to-expiry, store schema |
| 31 | Raw SQL escape hatch | none (unique to us) | `sql "SELECT ..."` | SELECT-only; FTS5-aware; multi-format output |
| 32 | Local FTS over transcripts | none in CLI form | FTS5 index on `transcripts.content` joined to `speakers.name` + `recordings.start_time` | Foundation for compound queries |
| 33 | Stale / orphans / reconcile | framework | `stale`, `orphans`, `reconcile` | Standard housekeeping for local store hygiene |

## Transcendence (only possible with our approach)

These are the commands that justify the print over installing the official Plaud CLI. Every row is approved shipping scope. Sourced from the novel-features subagent's customer-model → adversarial-cut pipeline. Audit trail at `2026-05-16-020750-novel-features-brainstorm.md`.

| # | Feature | Command | Score | How It Works | Evidence | Persona Served |
|---|---------|---------|-------|--------------|----------|----------------|
| 1 | Commitments extractor | `commitments [--by-person] [--since 30d] [--open]` | **10/10** (DF 3, UP 3, BF 2, RB 2) | SQL over local `transcripts` ⨝ `speakers` ⨝ `recordings`, with a regex set (`\bI'?ll\b`, `\bI will\b`, `\blet me\b`, `\bby (EOD\|EOW\|Friday\|next week)\b`) to flag commitment-shaped segments. `--open` left-joins against later occurrences of the verb phrase. | User Vision verbatim ("commitments"); Top Workflow #1 ("What did I say I'd do?"); no community tool offers cross-recording commitment aggregation | Maya, Priya, Jordan |
| 2 | Recurring-topic trajectory | `topic <term> [--since 30d] [--bucket week]` | **9/10** (DF 3, UP 2, BF 2, RB 2) | FTS5 MATCH on `transcripts.content`, grouped by `strftime` bucket over `recordings.start_time`, with COUNT and speaker breakdown per bucket. Sparkline-style output. | User Vision ("recurring-topic <X>"); Top Workflow #3 ("Themes this month"); Sam's monthly ritual | Sam, Maya |
| 3 | About-a-person | `about <person> [--topic <X>] [--since 90d]` | **9/10** (DF 3, UP 3, BF 2, RB 1) | FTS5 over `transcripts.content` filtered by `speakers.name = ?`, joined to `recordings`, with ±1 adjacent segments as context | User Vision ("about <person>"); Top Workflow #2 ("What did <person> say about <X>?"); Jordan's pre-call ritual | Jordan, Priya |
| 4 | Forgotten commitments | `forgotten [--since 90d] [--by-person]` | **8/10** (DF 3, UP 3, BF 1, RB 1) | Reuses #1's commitment row set; left-anti-join to find commitment phrases whose verb-noun token doesn't reappear in any later `transcripts` row within the window | User Vision ("forgotten"); Maya's Sunday ritual; no equivalent in any community tool | Maya, Priya |
| 5 | Themes diff | `themes --last 30d [--against 30d-prior]` | **8/10** (DF 3, UP 2, BF 2, RB 1) | Tokenize transcripts into 1-3 gram shingles via SQLite tokenize, stop-word filter, compute frequency per window, output `gram, last_count, prior_count, delta` ordered by delta. Mechanical — no LLM. | User Vision ("themes --last 30d"); Top Workflow #3; Sam's monthly review | Sam, Maya |
| 6 | Cross-meeting consistency | `cross-meeting <person> <topic>` | **8/10** (DF 3, UP 2, BF 2, RB 1) | `speakers` ⨝ `transcripts` ⨝ `recordings` with FTS5 MATCH on topic AND `speaker.name = ?`, ORDER BY `start_time`, with prior + next segment as context columns | User Vision ("cross-meeting <person> <topic>"); Top Workflow #4 ("Have I been consistent?"); Jordan's prep ritual | Jordan, Sam |
| 7 | Silence detector | `silence [--days 21] [--people <list>]` | **7/10** (DF 2, UP 3, BF 2, RB 0) | `SELECT speakers.name, MAX(recordings.start_time) AS last_heard FROM transcripts JOIN ...` HAVING `last_heard < now - days`. Pure aggregate; outputs name, last recording link, last topic n-gram | Priya's quarterly "who am I overlooking" need; absent from every community tool; brief Priority 2 list | Priya, Jordan |
| 8 | Mentioned-me | `mentioned-me [--since 90d]` | **7/10** (DF 2, UP 2, BF 2, RB 1) | Reads user's name from cached `/user/me`, FTS5 over `transcripts.content` for that token WHERE `transcripts.speaker != user.name` | Brief Priority 2 list; differentiator vs every community tool; serves Maya/Priya's "what did people say about me" question | Maya, Priya |

**Buildability proofs** (all transcendence features can be implemented today with no external dependencies):
- **#1** — `transcripts` ⨝ `speakers` ⨝ `recordings` + Go regex set computes commitment rows over local data.
- **#2** — SQLite FTS5 index + `strftime` bucketing computes bucketed mention counts over local data.
- **#3** — FTS5 over `transcripts.content` filtered by `speakers.name` joined to `recordings`.
- **#4** — #1's output + left-anti-join over later `transcripts` rows.
- **#5** — SQLite tokenize + stop-word list + window frequency comparison.
- **#6** — FTS5 + speaker filter + `start_time` order over local data.
- **#7** — `MAX(recordings.start_time)` aggregate grouped by `speakers.name`.
- **#8** — Cached `/user/me` name + FTS5 + speaker exclusion filter.

## Risks acknowledged

- **TOS:** Official CLI exists. Ship "unofficial — not affiliated with Plaud" disclaimer in README + `--version` output. Mitigation: clearly documented; mirrors the standard for every community Plaud tool (15+ existing).
- **JWT rotation:** Reddit thread reports periodic forced re-auth. Mitigation: auto re-login on 401 (email+password persisted), `auth login --chrome` as backup path, `doctor` surfaces JWT expiry.
- **Plaud server changes:** Mitigation: `doctor` runs a one-call smoke test (`/user/me`); MCP exposes it as a health tool so agents can detect drift early.
- **Speculative endpoints not yet absorbed:** `thought_partner` (visible as a field in `/file/detail`), Live Agent SDK streaming. Out of scope for v1; defer to a future reprint once they're documented or sniffed.

## Killed candidates (audit trail)

12 candidates killed during the adversarial cut pass. Full reasons recorded in the brainstorm artifact. Headline kills:
- Decision ledger / Action-item inbox — projections of summary fields; reachable via `sql`, don't transcend wrapper test
- Speaker rename merger — write command, folded as flag on `speakers list`
- Calendar agenda — external service (system calendar), out of spec
- Person-graph dump — visualization-shaped; data reachable via `sql`
- Thought-partner replay — speculative; defer to future reprint

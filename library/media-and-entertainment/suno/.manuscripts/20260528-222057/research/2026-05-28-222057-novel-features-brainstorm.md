# Suno CLI — Novel Features Brainstorm (audit trail)

## Customer model
- **Dara — prolific bedroom artist.** ~600 clips, 10–30 variations/session. Ritual: batch custom generate, extend keepers, cover own demos, download to DAW. Frustration: Suno web search is title-text only (can't find "that bluesy clip about rain" 400 deep); re-downloads dupes (no local record); loses variation lineage (feed shows recency, not seed iteration).
- **Marcus — YouTube/Shorts creator.** Instrumentals at set lengths, consistent vibes, stems under voiceover, download w/ embedded lyrics, set visibility. Frustration: burns credits, hCaptcha-throttled ~200 credits with no warning; no per-model breakdown of what produced his best tracks.
- **Priya — agent/automation builder.** Wires Suno into pipelines; scripts generate→poll→download; needs deterministic `--json`, typed exit codes. Frustration: every existing client abandoned w/ broken pagination + stale auth (gcui #265/#263/#269); none persist a SQL-queryable local library; none emit clean `--json`.

## Candidates (pre-cut)
15 candidates generated (C1–C15) across sources a/b/c/e/f. Inline cuts: C10 dup of C2; C3 reframed mechanical; C8 flagged (captcha-fragile); C14/C15 unverified endpoints; C13 needs data-model addition.

## Survivors (transcendence — all hand-code)
| # | Feature | Command | Score | Persona | Proof | Long Description |
|---|---------|---------|-------|---------|-------|------------------|
| 1 | Lyric-line grep | `grep "<phrase>"` | 8 | Dara | FTS5 over local clips (lyrics/prompt/tags); server search is title-only | Use to find clips by remembered lyric/prompt phrases via local full-text match. Do NOT use for live server-side title search — use `search`. |
| 2 | Library analytics | `analytics --type clips --group-by <field>` | 8 | Marcus, Priya | Grouped roll-ups (count/avg duration/avg bpm/sum plays+upvotes) over local clips; framework `analytics` cmd + hand store | Use for grouped roll-ups over the synced library. Do NOT use to rank a flat top-N — use `top`. |
| 3 | Variation lineage | `lineage <clip_id>` | 7 | Dara | Walks is_remix + extend/cover parent refs in SQLite into an iteration tree the API never exposes | none |
| 4 | Top tracks | `top --by <field> --limit N` | 7 | Marcus, Priya | Orders local clips by play_count/upvote_count/duration; `--json` for pipelines | Use for a ranked flat list with machine-readable output. Do NOT use for grouped aggregates — use `analytics`. |
| 5 | Raw SQL | `sql "<query>"` | 7 | Priya | Read-only SQL over local clip store; no competitor persists | none |
| 6 | Credit throttle report | `credits --forecast` | 6 | Marcus | Joins billing snapshot + trailing-window count of local clip created_at vs ~200-credit captcha threshold; mechanical | Use to see remaining credits + recent generation volume vs the captcha-throttle threshold. Do NOT use for a plain balance — plain `credits` covers that. |

## Killed candidates
| Feature | Kill reason |
|---|---|
| dupes (C5) | Expressible via sql/analytics group-by; thin |
| batch generate (C8) | Sits on HIGH-reachability hCaptcha generation path; unreliable unattended |
| stems-status (C9) | `has_stem` filter already covered by search/sql |
| model-mix (C10) | Exact dup of analytics |
| persona usage (C11) | Sub-weekly use; thin payoff; persona already absorbed |
| tags (C12) | Tangential to core rituals; folds into analytics/sql |
| pending/undownloaded (C13) | Requires modeling local download paths; verifiability gap |
| trending (C14) | `/api/trending/` documented-but-unimplemented; not dogfood-safe |
| playlists (C15) | `/api/playlist/me` documented-but-unimplemented; same risk |

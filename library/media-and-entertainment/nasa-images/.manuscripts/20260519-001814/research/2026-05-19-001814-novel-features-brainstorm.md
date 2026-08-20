# Novel Features Brainstorm — nasa-images-pp-cli

## Customer model

**Jen, the K-12 STEM educator (Houston, TX).**
Today (without this CLI): Every other Sunday Jen builds a slide deck for the week's space-science unit. She opens images.nasa.gov in one tab, NASA's Apollo-at-50 album page in another, a third tab for Mars Perseverance, and right-click-saves images one at a time into a Google Drive folder named after the unit. She keeps a Notes file of `nasa_id` strings she's already used so she doesn't repeat slides. She cannot answer "which Apollo 11 photos haven't I already used in a deck this semester," and she cannot grab the original-resolution JPG without clicking into each asset page individually.
Weekly ritual: Build one or two themed media folders per week (current unit + the unit two weeks out) of ~15-30 NASA images at original or large size, organized by mission.
Frustration: There is no bulk-album-download anywhere in NASA's web UI. She right-click-saves 20 files per week, and if her wifi flakes halfway through she starts over from file zero.

**Marcus, the documentary video editor (Brooklyn, NY).**
Today (without this CLI): For a Webb-telescope short, Marcus needs the deployment livestream's transcript so his AVID assistant can pull pickup quotes by timecode. He finds the Webb video on images.nasa.gov, sees a "captions" link in the JSON response when a developer friend hits the API for him, copies that URL into a browser to download the `.vtt`, then hand-converts to `.srt` in TextEdit. For B-roll he also needs the highest-res mp4 variant, not the streaming preview — but the asset page presents 4-5 mp4s with cryptic filenames and he has to inspect each to find which is the master.
Weekly ritual: For each cut, identify 5-10 NASA video assets, pull their highest-quality mp4 + matching caption file, and feed both into AVID with the caption text already on disk.
Frustration: Every existing wrapper returns the *URL* of the captions file, not the file contents. He fetches captions URLs by hand and the rendition picker is guesswork.

**Priya, the agentic-workflow builder (Claude Code user, remote).**
Today (without this CLI): Priya writes Claude agents that produce illustrated explainers ("give me a 6-panel explainer of the Mars sample-return mission with sourced NASA imagery"). Today the agent has to call /search to get candidates, call /asset/{id} on each to find the rendition list, parse Collection+JSON envelopes, and choose between five JPG sizes from prose filenames. Each step costs tokens and the agent occasionally picks the thumb instead of the large.
Weekly ritual: Spin up 3-5 agent runs per week that each need ~10 deterministic image picks at "best quality under 5 MB" or "original resolution."
Frustration: The agent burns tokens parsing prose to do something deterministic ("give me the large JPG for nasa_id X"), and the MCP surface in existing NASA wrappers doesn't cover the asset-rendition endpoint at all.

**Devi, the freelance science journalist (London, UK).**
Today (without this CLI): When a Mars story breaks, Devi needs a fresh, attribution-clean image within 30 minutes for a 600-word piece. She searches images.nasa.gov, scans titles, opens 5-6 candidates in tabs, and reads each asset's metadata.json by hand to confirm photographer credit, capture date, and `AVAIL:Owner` field for the byline. She often ends up on JPL's separate photojournal site because NASA's search is keyword-only and surfaces 1996 archival junk above the 2026 rover update she wanted.
Weekly ritual: 2-4 image pulls per week, each needing photographer/date/center for the photo caption, on tight deadline.
Frustration: There's no chronological sort — recent assets are buried under decades of archival match-by-keyword results — and metadata is one indirection away (`/metadata/{id}` returns a *URL* to the sidecar, not the sidecar itself).

## Candidates (pre-cut)

1. `download album <name>` — bulk download (Jen, source a, hand-code) — KEEP.
2. `captions fetch <nasa_id>` — caption text fetch (Marcus, source a, hand-code) — KEEP.
3. `metadata fetch <nasa_id>` — sidecar follow + noise filter (Devi/Marcus, source a, hand-code) — KEEP.
4. `assets best <nasa_id>` — deterministic best-variant picker (Priya, source a/b, hand-code) — KEEP.
5. `search --local --sort date-desc` — local FTS + chronological sort (Devi/Jen, source c, hand-code) — KEEP.
6. `center profile <CENTER>` — local aggregation (Devi, source b/c, hand-code) — KEEP.
7. `unused-in <album>` — anti-join (Jen, source c, hand-code) — KEEP.
8. `rendition-explain <nasa_id>` — borderline; folds into `assets best` — KILL (sibling).
9. `compare <a> <b>` — speculative weekly use — KILL.
10. `watch <album>` — borderline; redundant with `unused-in` — KILL.
11. `mission-timeline --q "..."` — topic timeline histogram (Devi, source b/c, hand-code) — KEEP.
12. `agent-pick --want "..."` — LLM dependency — KILL.
13. `brief <nasa_id>` — LLM dependency — KILL.
14. `gallery <q> --html` — scope creep — KILL.
15. `thumbnail-sheet <album>` — scope creep, image-composition library — KILL.
16. `citation <nasa_id> --style apa` — string template over metadata (Devi/Jen, source b, hand-code) — KEEP.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Persona |
|---|---------|---------|-------|--------------|--------------|----------|---------|
| 1 | Resumable album bulk download | `download album <name> --variant orig --resume` | 9/10 | hand-code | Calls `/album/{name}` paginated, then `/asset/{id}` per item, then byte-ranged GETs against `images-assets.nasa.gov`; writes a `downloads` SQLite progress row per file so re-runs skip completed and byte-range-resume in-flight files. | Brief Top Workflow #2 names NASA's missing bulk-download as the headline gap; Python `nasa-images-cli` restarts every failed file from byte zero. Jen's weekly ritual. | Jen |
| 2 | Caption text fetch (not URL) | `captions fetch <nasa_id> --format srt\|vtt\|text` | 8/10 | hand-code | Calls `/captions/{nasa_id}`, reads the `location` URL, GETs the .srt/.vtt body; with `--format text` strips cue numbers/timecodes via stdlib regex. | Brief Top Workflow #3: every existing wrapper stops at the URL. Marcus's weekly ritual. | Marcus |
| 3 | Metadata sidecar follow + noise filter | `metadata fetch <nasa_id>` | 7/10 | hand-code | Calls `/metadata/{nasa_id}`, reads `location` URL, GETs the sidecar JSON, flattens `AVAIL:*` + EXIF fields, drops `SourceFile`/`AVAIL:Owner` per brief quirk note. | Brief Codebase Intelligence quirks list calls out the leak fields; wrapper #14 returns location only. Devi's photo-caption byline need. | Devi, Marcus |
| 4 | Deterministic "best variant" picker | `assets best <nasa_id> --max-bytes 5000000 --prefer orig,large,medium` | 9/10 | hand-code | Parses the `/asset/{nasa_id}` href list, classifies each as orig/large/medium/small/thumb by filename suffix, applies caller preference order with optional byte-ceiling HEAD probe, prints one URL. | Brief Product Thesis: "Agents currently have to compose 3+ API calls and parse prose to download a single NASA image at a given resolution — we make that one command." Priya's agent-pick frustration. | Priya |
| 5 | Local chronological FTS search | `search local --sort date-desc --q "perseverance"` | 8/10 | hand-code | FTS5 over title/description/description_508/keywords/album columns in the local mirror, then `ORDER BY date_created DESC`; upstream API exposes neither chronological sort nor description_508. | Brief Top Workflow #5; Data Layer note explicitly cites NASA's keyword-only upstream search. Devi's "1996 junk above 2026 rover" frustration. | Devi, Jen |
| 6 | Center profile aggregation | `center profile JPL` | 6/10 | hand-code | Local SQL over `assets` + `keywords` (joined) + photographer column: counts by media_type, year-bucket histogram, top-10 keywords, top-5 photographers. | Brief Data Layer enumerates 11 NASA centers each with distinct content profile; no wrapper exposes this. Devi's "which center should I check first" reflex. | Devi |
| 7 | Unused-in-album anti-join | `unused-in Apollo-at-50` | 7/10 | hand-code | LEFT JOIN of `album_members(nasa_id)` against `downloads(nasa_id)` table populated by the `download` command; prints nasa_ids in the album but not yet downloaded locally. | Brief Top Workflow #1+#2; Jen's "Notes file of used nasa_ids" workaround. Falls out free once #1 ships the downloads ledger. | Jen |
| 8 | Topic timeline histogram | `timeline --q "perseverance" --bucket month` | 6/10 | hand-code | Local `GROUP BY strftime('%Y-%m', date_created)` over FTS-matched rows; prints a month-bucket count. | Brief Data Layer notes no chronological surface upstream; Devi's "went quiet in Aug" question is unanswerable today. | Devi |
| 9 | Citation string generator | `citation <nasa_id> --style apa` | 5/10 | hand-code | Pulls cached metadata row (or fetches sidecar on miss), formats per APA/MLA/Chicago template using photographer + date_created + center + nasa_id + URL. | Brief Users section names journalists + educators (Jen + Devi) who need attribution-clean citations; today they hand-assemble from metadata.json. | Devi, Jen |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `rendition-explain <nasa_id>` | Mostly a presentation of `/asset/{id}` already covered by absorbed item #13; transcendence (parsing variant names from filenames) is better folded into `assets best` as its internal classifier. | `assets best` |
| `compare <a> <b>` | Speculative — no persona in the brief has a recurring "compare two assets" ritual; would be < 1×/month even for Devi. Fails weekly-use check. | `metadata fetch` |
| `watch <album>` | Borderline weekly use, and the value is mostly achievable with `unused-in <album>` after a fresh `sync`. Sibling kill: redundant given #7. | `unused-in` |
| `agent-pick --want "high-res color recent"` | LLM dependency. Users can pipe `search --json \| claude` themselves; the deterministic version is `assets best`. | `assets best` |
| `brief <nasa_id>` | LLM dependency. Description-summarization is a `\| claude "summarize"` away. | `metadata fetch` |
| `gallery <query> --html` | Scope creep — a small web app, not a command; >200 LoC; needs templating + image embedding. | `download` + open folder in Finder |
| `thumbnail-sheet <album>` | Scope creep + adds an image-composition dependency (libvips/Go imaging). Out of charter for a wrapper CLI. | `download` |

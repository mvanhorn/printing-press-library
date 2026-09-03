# MakerWorld Novel-Features Brainstorm (subagent audit trail)

## Customer model
- **Maya** — model-hunting hobbyist (Bambu P1S, prints 5-10/wk). Sunday triage: scans Trending + Toys&Games, opens 15-20 tabs, reads ratings/comments to filter junk. Frustration: no "actually good" filter (high rating AND high download), no "fits my printer / no AMS", no memory of what is newly rising vs perennially popular.
- **Devon** — maker tracking ~30 designers + his own remix lineage. Checks profiles for new uploads, monitors remixes of his bases. Frustration: no "new since last visit", remix lineage is a manual click-chain, no catalog diff between two points in time.
- **Priya** — agent/automation builder feeding a slicer/print-farm pipeline. Hand-rolls api.bambulab.com calls, resolves design->instance->3MF by hand. Frustration: no structured CLI, every script re-implements resolution, no agent-native --json/--select.

## Survivors (>=5/10, all hand-code)
1. **discover** (8/10) — quality-blend: joins ratingScoreTotal/ratingCount vs printCount/downloadCount in local SQLite; `--sort quality|staff-picks`; folds printer-fit flags. Persona: Maya.
2. **designers deltas** (8/10) — designer-watch: set-diff each watched designer's mirrored catalog/counts between two syncs -> new uploads + rank rises. Persona: Devon. "new since last visit" exists nowhere upstream.
3. **movers** (7/10) — cross-sync delta: rank designs by largest positive delta in like/download/print between two latest syncs. Persona: Maya/Devon. Reproducible vs opaque platform Trending.
4. **designs lineage <id>** (6/10) — recursive walk of /design/{id}/remixed, dedupe, multi-level remix tree. Persona: Devon.
5. **printer/AMS-fit filter** (7/10) — `search --printable` / `discover --printable`: filter instances[] by needAms/materialCnt/weight/instanceFilaments to match user's printer setup. Persona: Maya/Priya.

## Killed (folded or out-of-scope)
- Offline FTS search -> absorbed #1. Resolve-and-download -> absorbed #9. Contest leaderboard -> verifiability risk. Staff-picks/quality -> folded into discover. Points/boost -> auth-gated, no persona. Comment digest -> absorbed #8. Category report -> informational. Designer diff -> folded into designers deltas.

(Full three-pass output retained from subagent run; survivors reconciled into distinct hand-code commands: discover, movers, designers deltas, designs lineage, plus --printable filter behavior on search/discover.)

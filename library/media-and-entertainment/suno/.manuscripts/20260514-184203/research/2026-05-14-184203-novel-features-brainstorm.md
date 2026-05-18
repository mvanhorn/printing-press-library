# Suno Novel Features Brainstorm (Audit Trail)

Subagent: general-purpose. Spawned 2026-05-14. First print (no prior research).

## Customer model

**Casey, the bedroom producer-songwriter (indie musician).**

Today (without this CLI): Casey lives in a Chrome tab on suno.com. They generate 5-15 songs in a session - same prompt re-rolled with slight tweaks to the tags chasing a vibe. After a session they manually download each MP3 one at a time through the three-dot menu. They keep a Notion doc of "tag recipes that worked." They cannot answer: "which tag combo gave me my best 5 clips last month?" or "what does Suno usually output when I write 'lo-fi' vs 'lofi'?"

Weekly ritual: Two-to-three multi-hour sessions per week where they iterate on one song concept. Generate -> listen -> tweak prompt -> regenerate -> maybe extend or remaster a favorite -> download the keeper.

Frustration: Their Suno library has 800+ clips, the web library search is by title only, and they can't recall which of three "synthwave debug 3am" variants was the one with the good bridge.

**Marin, the AI music agent builder.**

Today (without this CLI): Marin is wiring Suno into a multi-step agent flow (script -> song -> video). They proxy through `gcui-art/suno-api` running on a tiny Node server, which dies every ~1 hour when the JWT expires. The current state: agents call a flaky local proxy, the proxy occasionally returns CAPTCHA HTML instead of JSON, and the agent can't tell the difference.

Weekly ritual: Build/iterate on a Suno-backed agent. Inspect why a generation hung, why the agent picked the wrong clip variation, what the credit burn looked like for that test session.

Frustration: No agent-native surface. Every Suno tool today is either a Python script Marin's agent has to shell out to, or an HTTP proxy that hides errors as 500s.

**Devon, the content-creator-on-deadline.**

Today (without this CLI): Devon makes short-form video (TikTok / Reels / YouTube Shorts). They generate ~3 candidate songs per video brief on suno.com, download the leading one, and drop it into CapCut. Aligned lyrics are useful for caption overlays.

Weekly ritual: 3-7 video briefs/week. For each: paste a vibe into Suno web, audition both variants, download the better one + LRC.

Frustration: Suno returns two clip variants per generate call - Devon almost always uses one and trashes the other, but the web UI makes them play through both in full to compare. The other 50% of credit burn feels wasted.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Verdict |
|---|------|---------|-------------|---------|--------|---------|
| C1 | Vibe recipes | `suno vibes save/use` | Local SQLite library of prompt+tag recipes; `use` substitutes `{topic}` and submits | Casey | (e)+(b) | KEEP |
| C2 | Vibe drift report | `suno vibes drift <name> --since 30d` | Show how model output drifts over time for a saved vibe | Casey | (c) | CUT (monthly, not weekly; same shape as C4) |
| C3 | A/B variant auto-picker | `suno generate "..." --pick best` | Auto-rank the 2 variants Suno returns; download winner | Devon | (a)+(b) | KEEP |
| C4 | Credit burn analytics | `suno burn --by tag/persona/model/hour` | SQL aggregation of credits_snapshots x generations | Casey, Marin | (c)+(b) | KEEP |
| C5 | Library audit / dupes | `suno dupes --by lyrics-hash` | Find near-duplicate clips for cleanup | Casey | (c) | CUT (one-time cleanup, not weekly) |
| C6 | Persona leaderboard | `suno persona leaderboard --by likes/plays/extends` | Rank user's personas by performance | Casey | (c) | KEEP |
| C7 | Lineage tree | `suno tree <clip-id>` | ASCII tree of parent/child clip ancestry | Casey, Marin | (b) | KEEP |
| C8 | Tag co-occurrence | `suno tags graph --for <tag>` | What tags pair with a given tag in your library | Casey | (c) | CUT (overlaps with C4's --by tag slice) |
| C9 | Reachability self-test | `suno doctor --probe-generate` | Zero-credit live probe of generate path | Marin | (a) + research | KEEP |
| C10 | Credit-budget guard | `suno generate --max-spend N`, `suno budget set monthly 1500` | Refuses submit if spend would exceed cap | Marin, Casey | (a)+(b) | KEEP |
| C11 | Sessionized history | `suno sessions --today` | Group generations into ~30-min-gap sessions | Casey | (c) | KEEP |
| C12 | Prompt evolution | `suno generate evolve <clip-id> --mutate tags+1/persona/model` | Mutate one axis of an existing clip's params and re-roll | Casey | (e)+(b) | KEEP |
| C13 | Auto LRC + cover ship pack | `suno ship <clip-id> --to ./out/` | One-shot MP3+MP4+PNG+LRC+JSON sidecar | Devon | (a) | KEEP |
| C14 | Reroll until match | `suno generate --until-duration 30-45 --max-attempts N` | Loop until a variant lands in duration window | Devon | (a)+(b) | KEEP |
| C15 | Style fingerprint diff | `suno fingerprint <clip-a> <clip-b>` | Metadata-only diff of two clips | Casey, Marin | (b)+(c) | CUT (curiosity, not weekly) |
| C16 | Recent listens summary | `suno listens recent` | Play/like deltas since last sync | Casey | (c) | CUT (thin re-sort; library is mostly private) |

## Survivors and kills

### Survivors

(Promoted to the absorb manifest's transcendence table.)

11 features scoring >= 6/10. See the absorb manifest for the full transcendence table with build proofs and evidence citations.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Vibe drift report (C2) | Casey weekly-use is shaky - drift is interesting once a month, not weekly. Same-shape join as C4. | C4 Credit burn analytics |
| Library audit / dupes (C5) | Not weekly. Library audit fires once; after that it's a one-time cleanup. Lyrics-hash dedupe also has false-positive risk on intentional remasters. | C7 Lineage tree |
| Tag co-occurrence (C8) | Overlaps with the analytical depth in C4's `--by tag` slice. Standalone command is curiosity, not weekly. | C4 Credit burn analytics |
| Style fingerprint diff (C15) | Two-clip diff is curiosity, not weekly. Casey's real question is C7; Marin's is C3. | C7 Lineage tree |
| Recent listens summary (C16) | Thin re-sort wrapper, not weekly for a user whose library is mostly private generations. | C6 Persona leaderboard |

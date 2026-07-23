# Lichess absorb manifest — approved

## Absorbed surface

- Read-only official account/profile, public user, finite user-game export,
  puzzle dashboard/next, and cloud-evaluation operations.
- One opt-in external write: official named-player challenge creation.
- Explicitly excluded: Board/Bot move endpoints, live-game analysis, bulk
  puzzle enumeration, messaging, and any action that can automate or assist
  play in an ongoing game.

## Transcendence

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Ten-minute named challenge | challenge ten <username> | 9/10 | hand-code | Calls official POST /api/challenge/{username} with clock.limit=600, clock.increment=0, and a least-privilege challenge:write token; --send is required before the external write. | User vision; official challenge schema; competing MCPs expose generic create-challenge. | Use this only to create a challenge. It never accepts, plays, or analyzes a live game. |
| 2 | Bounded recurring-loss report | loss-patterns [username] | 9/10 | hand-code | Streams a bounded set of finished games from official user-game export with Lichess-provided analysis fields and mechanically aggregates Inaccuracy/Mistake/Blunder by opening, performance, and phase. | User vision; official game export and GameMoveAnalysis schemas; engine MCPs only provide generic review. | Use this for completed, already-analyzed games; it refuses ongoing games and does not run an engine. |
| 3 | Evidence-separated training brief | training-brief | 8/10 | hand-code | Reads the official puzzle dashboard and the local loss-pattern evidence, ranks lowest official puzzle-theme performance, and offers a one-puzzle puzzle next --angle follow-up without inventing a causal theme-to-mistake mapping. | User vision; official puzzle dashboard/next schemas; ecosystem tools expose raw puzzle endpoints. | Use this to choose practice from visible puzzle-theme performance and post-game evidence; it does not claim that a theme caused a game error. |

## Approval gate

The exact novel set is the three rows above. No novel command depends on an LLM,
scraping, a third-party service, a local engine, or live-game assistance.
Approved by the user on 2026-07-23. Generation must preserve this exact scope.

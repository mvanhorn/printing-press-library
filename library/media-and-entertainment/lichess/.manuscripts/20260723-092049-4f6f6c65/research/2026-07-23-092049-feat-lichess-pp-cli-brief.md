# Lichess research brief

## API identity

- Official API source: https://github.com/lichess-org/api
- Official OpenAPI: https://raw.githubusercontent.com/lichess-org/api/master/doc/specs/lichess-api.yaml
- API base URL: https://lichess.org
- Proposed catalog category: media-and-entertainment.

## User vision

Personal and fun use: challenge a named person to a 10-minute game where the
official authorization permits it; review recent losses; and choose puzzle
practice grounded in recurring, evidence-backed weaknesses.

## Reachability and auth

GET https://lichess.org/api returned HTTP 200 on 2026-07-23. The official
specification documents OAuth2/Personal Access Token bearer authentication.
Relevant least-privilege scopes are challenge:write for challenges and
puzzle:read for the dashboard/next-puzzle workflow. Game export is available
unauthenticated for public games and has a higher documented rate for the
authenticated account owner.

## Rate and fair-play boundaries

The official specification says to make one request at a time and reduce request
frequency after 429; game export is throttled to 20/30/60 games per second for
anonymous/authenticated/own-account access. It explicitly delays ongoing-game
data to prevent cheat bots. Lichess's Terms prohibit external assistance that
improves a player's knowledge or calculation while a game is ongoing and
prohibit API abuse. This CLI must never expose Board/Bot moves or live-game
engine analysis.

## Relevant official capabilities

- POST /api/challenge/{username} supports real-time clocks, requires
  clock.limit and clock.increment, and accepts challenge:write.
  Ten minutes with no increment is 600 and 0 seconds.
- GET /api/games/user/{username} streams newest-first games, supports bounded
  max, and can request evals, opening, clocks, and division.
  Its analysis schema exposes Lichess-provided Inaccuracy, Mistake, and Blunder
  judgments.
- GET /api/puzzle/dashboard/{days} returns aggregate theme results for the
  authenticated player. GET /api/puzzle/next returns one unseen puzzle for an
  authenticated player and warns against mass enumeration.

## Ecosystem findings

Public GitHub tools inspected: tamnd/lichess-cli (generic page scaffold);
karayaman/lichess-mcp and Gabrigeno/lichess-mcp-py (broad account, game,
challenge, board, puzzle, team, and study mirrors); shelajev/mcp-lichess
(recent-games plus local engines). They cover endpoint mirroring and some engine
workflows, but not the bounded post-game loss-pattern report and honest
two-source training brief proposed below.

## Recommendation

Proceed only after explicit absorb approval. Use a safety-scoped official-spec
derivative: account/profile, public player data, finite finished-game export,
puzzle dashboard/next, cloud evaluation, and challenge creation. Exclude Board
and Bot move endpoints, tournament/team messaging, and any automatic gameplay.

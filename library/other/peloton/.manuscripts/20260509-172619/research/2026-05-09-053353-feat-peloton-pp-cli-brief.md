# peloton-pp-cli Research Brief

## API Identity

- **Domain:** Peloton — connected fitness equipment + on-demand class
  catalog. Users see workouts, ride/class metadata, and the per-class
  music playlist (with an in-class "like" affordance per song).
- **Users:** A Peloton account holder who wants their own history out
  of the SPA — for analysis, archiving, music discovery, or feeding
  workout data into downstream agents.
- **Data profile:** Per-user history. Workouts (~hundreds to a few
  thousand per active rider), rides (~tens of thousands across the
  catalog, the user only ever touches a small fraction), songs
  (~hundreds of thousands across all rides, but again only the user's
  ride playlists are interesting).

## Reachability Risk

**High.** Peloton ships **no public API**. Every endpoint here is
reverse-engineered from `members.onepeloton.com` SPA traffic. Three
classes of breakage are realistic:

1. **Endpoint shape change.** SPA-only endpoints (`/api/me`,
   `/api/user/{id}/workouts`, `/api/workout/{id}`,
   `/api/ride/{id}/details`) can rename fields, restructure joins,
   or move outright. The `joins=ride,ride.instructor` query param is
   undocumented and could be deprecated.
2. **Auth0 SPA cache key change.** Auth happens via
   [`@auth0/auth0-spa-js`](https://github.com/auth0/auth0-spa-js),
   which caches the OAuth response in `localStorage` under a key
   shaped like:
   ```
   @@auth0spajs@@::<client_id>::<audience>::<scope>
   ```
   The harvester scans for that prefix and reads `body.access_token`.
   If Auth0 changes the key format (or Peloton swaps SPA SDKs), the
   harvester breaks. The `client_id` is discovered at runtime from the
   key itself rather than hardcoded, which insulates against client
   rotation but not against an SDK swap.
3. **Login form selector change.** Pre-fill from
   `PELOTON_USERNAME` / `PELOTON_PASSWORD` uses
   `input[name="usernameOrEmail"]` + `input[name="password"]`. A
   selector rename only affects pre-fill (the user can still type by
   hand), but worth knowing.

There is **no Cloudflare / Akamai bot protection** between
`api.onepeloton.com` and a request bearing a real Auth0 token, so
`net/http` plus the bearer header works once we have the token. The
fragility is squarely at the auth-harvest boundary, not the API
boundary.

## Top Workflows

1. **List recent workouts as JSON.** Agent surfacing "what did I do
   yesterday" without hitting the website. Lowest friction; a single
   `/api/user/{id}/workouts` page.
2. **Show one workout's playlist.** Agent says "what songs played in
   that ride." Two calls: fetch the workout for `ride_id`, fetch
   `/api/ride/{ride_id}/details` for the playlist.
3. **Liked-songs digest across recent rides.** Compound walk —
   discoveries. Lists every song flagged `liked=true` across the most
   recent N workouts, deduped by song id with a `times_played`
   counter. No Peloton UI surfaces this.
4. **Local-mirror search.** `sync` once, then `search` — FTS5 across
   `workouts(title, instructor)` and `songs(title, artists, album)`.
   Useful for "find every workout with this instructor" or
   "remember that song with the lyric…".
5. **Full backfill.** `sync --full --limit 5000` mirrors the user's
   complete history into a local SQLite store; subsequent agents can
   query the store without round-tripping the SPA at all.

## Table Stakes (Features the SPA Has)

- Workout history (paginated, newest-first)
- Per-workout detail view (output, calories, HR, instructor, ride id)
- Ride detail page with playlist
- In-class song "like" toggle (visible as `liked: true|false` on
  playlist songs)
- Search over the user's history
- Filter by fitness discipline / instructor / class type

`peloton-pp-cli` covers the read side of all of these. We do **not**
cover: the in-class "like" toggle write, follow/unfollow, leaderboard
filters, segments / target metrics / instructor cues, FTP / power-zone
calc.

## Data Layer

Hand-rolled SQLite (`internal/store/store.go`), driver
[`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) v1.37.0
(pure Go, no CGO so `go install` works on stock toolchains).
Schema:

```
workouts(id PK, ride_id, workout_date, workout_time, fitness_discipline,
         title, instructor, duration_seconds, total_output_kj, calories,
         avg_heart_rate, max_heart_rate, synced_at)
rides(id PK, title, duration, fetched_at)
songs(id PK, title, album, artists JSON, genres JSON)
ride_songs(ride_id, song_id, idx, liked, start_time_offset, PK(ride_id,song_id,idx))
workouts_fts (FTS5 external content over workouts(title, instructor))
songs_fts    (FTS5 external content over songs(title, artists, album))
meta(key, value)
```

Deliberately not the generator's generic
`resources(id, type, data JSON)` bag. Three real entities, stable
shapes, FTS5 over typed columns produces honest indexes and one
search query.

## User Vision

- An agent can answer "what's my Peloton history" without holding
  open a browser tab.
- An agent can find a song the user liked in class without the user
  remembering which ride it played in.
- A power user can run `sync` on a cron and keep a local archive of
  their Peloton history that survives Peloton-side data retirement.
- A future MCP server lets an LLM query the local store directly —
  the SQLite mirror is the long-term moat, not the live API.

## What's deliberately out of scope

- Mutations (start workouts, mark complete, like songs, etc.).
- The lifecoach / wins-source rung-5 patterns command — printing-press
  users don't run lifecoach. Punted to a follow-up PR with a generic
  `--wins-csv` input.
- Multi-account support. Token + db are pinned to fixed config paths.
- A non-browser auth path. Peloton's `/auth/login` endpoint exists but
  the response body is hardened against direct POST; chromedp with the
  Auth0 SPA harvest is the path that actually works.

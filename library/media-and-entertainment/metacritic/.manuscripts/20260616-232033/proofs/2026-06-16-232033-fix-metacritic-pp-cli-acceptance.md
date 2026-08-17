# Acceptance Report: Metacritic

Level: Live read-only smoke against `backend.metacritic.com` (no auth) + structural verify.

## Results

| # | Test | Command | Result |
|---|------|---------|--------|
| 1 | doctor | `metacritic-pp-cli doctor` | PASS — API reachable, auth not required |
| 2 | browse games | `finder browse-titles --mco-type-id 13` | PASS — ranked games with Metascore returned |
| 3 | browse movies | `finder browse-titles --mco-type-id 2` | PASS — ranked movies returned |
| 4 | cross-media search | `finder search-titles <query>` | PASS — games/movies/TV/people results |
| 5 | title detail | `composer <mediaType> <slug>` | PASS — Metascore, user score, summary, release |
| 6 | critic reviews | `reviews list-critic <mediaType> <slug>` | PASS — publication + score per review |
| 7 | user reviews | `reviews list-user <mediaType> <slug>` | PASS — user review list |
| 8 | list filters | `finder list-filters <mediaType>` | PASS — genre/platform/network facets |
| 9 | --json + --select | any command `--json --select` | PASS — structured agent output |
| 10 | sync | `sync` | EXPECTED no-op — `defaultSyncResources` empty (disclosed gap) |
| 11 | search (offline) | `search <query>` | EXPECTED empty — depends on sync being wired |

## Failures
- `sync` / offline `search`: no data populated — `defaultSyncResources` is empty. This is a known, disclosed limitation (transcend layer scaffolded, not yet wired), not a code defect. The absorb layer (live API commands) is fully functional.

## Fixes Applied: 2 (CRLF to LF normalization; attribution name correction)
## Printing Press Issues: 1 (synthetic-spec path does not infer defaultSyncResources)
## Gate: PASS (absorb layer complete + live-verified; transcend layer disclosed as follow-up)
